# 生产数据库恢复

用 `.github/workflows/restore-prod-db.yml`（Restore Production Database）把最终备份恢复到生产 PostgreSQL，然后用常规 Deploy 工作流启动应用。

## 工作流做什么

手动触发（`workflow_dispatch`，无输入），在 `prod` environment、`[self-hosted, multica-prod-qwip]` Runner 上执行：

1. 确认 `GITHUB_WORKSPACE` 和 `RUNNER_TEMP` 都在 `/data` 下，否则直接失败。
2. checkout `v0.2.126`，复用其中的 `docker-compose.prod.yml`。
3. 在 `$RUNNER_TEMP` 下创建 0700 临时目录，用 `umask 077` 写临时 `.env.prod`。只写 Compose 渲染所需的 prod Secrets（`POSTGRES_PASSWORD`、`JWT_SECRET`、`FRONTEND_ORIGIN`），以及固定值：
   - `PGDATA_DIR=/data/multica/data`
   - `LISTEN_PORT=8888`
   - `IMAGE_TAG=v0.2.126`
4. 如果 `multica-prod` 项目里除 `postgres` 以外还有服务在运行，直接失败。
5. 校验 `/data/multica/backup/multica-prod-final-20261109-181756.dump.sha256`：文件只能有一条记录，且指向对应 dump；SHA-256 必须一致；`pg_restore --list` 必须能读取归档。
6. 只启动 `postgres`（`up -d --no-deps postgres`），确认数据挂载是 `/data/multica/data`，然后通过 TCP 等待就绪。
7. 用 prod 密码经 TCP 登录，确认目标库 `multica` 为空：没有用户表、视图、序列等对象，没有 `public` 以外的 schema，没有 `plpgsql` 以外的扩展。不为空就失败，不做任何改动。
8. 执行 `pg_restore --no-owner --no-acl --single-transaction --exit-on-error`。任何错误都会让整个事务回滚。
9. 检查 `public` schema 下的表数量，为 0 则失败。
10. 无论成功失败都删除临时目录（包括 `.env.prod`）。

工作流不会启动 migrate、backend、frontend、nginx，也不会删除任何数据或卷。它与 Deploy 共用 `deploy-production` concurrency group（`cancel-in-progress: false`），不会和生产部署并发。

### 备份文件权限

备份文件是 `root:root 0600`，Runner 用户读不了，而且不假定有 sudo。所有读取备份的操作都放在一次性容器（`pgvector/pgvector:pg17`，容器内是 root）里完成，`/data/multica/backup` 以只读方式挂载。备份文件的属主和权限不会被修改。

`pg_restore` 容器使用 `--network container:<postgres>`，通过 `127.0.0.1` 连接 postgres，不对宿主机暴露端口。密码通过 `PGPASSWORD` 环境变量传入，不出现在命令行或日志里。

## 前置条件

- Runner 用户能使用 Docker（例如在 `docker` 组里）。
- Runner 的工作目录和临时目录在 `/data` 下。
- `multica-prod` 的应用服务都已停止。
- `/data/multica/data` 不存在、为空目录，或者是一个只有空 `multica` 库的集群。如果它已经有一个由其他密码初始化的集群，第 7 步会因认证失败而停止。

## 恢复成功后：常规生产部署

恢复成功后，`postgres` 保持运行，数据在 `/data/multica/data`。接下来运行常规 Deploy 工作流：

1. 在生产主机上停止占用 8888 端口的备份下载服务。必须在启动 nginx 之前完成，否则 nginx 绑定端口会失败：

   ```bash
   sudo systemctl stop multica-backup-http.service
   ss -ltn 'sport = :8888'   # 应没有输出
   ```

2. 确认 `prod` environment 的变量仍是：
   - `PGDATA_DIR=/data/multica/data`（与恢复时相同；Deploy 从 `vars.PGDATA_DIR` 读取）
   - `LISTEN_PORT=8888`

3. 触发 Deploy，`version` 填 `v0.2.126`，**`clean` 保持 `false`**：

   ```bash
   gh workflow run deploy.yml --repo nullne/multica -f version=v0.2.126 -f clean=false
   ```

   `clean=true` 会删除 `PGDATA_DIR`，也就是刚恢复的数据，绝对不要使用。

Deploy 会沿用正在运行的 `postgres`，执行 migrate（对已恢复的库是增量的），再启动 backend、frontend、nginx，并在 `http://localhost:8888/health` 上做健康检查。
