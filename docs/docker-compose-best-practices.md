# Docker Compose Production Best Practices

通用的 Docker Compose 生产部署实践，适用于任何 Web 应用项目。

---

## 1. 端口暴露：最小化攻击面

**原则：Prod 只暴露一个入口端口，其他服务全部内网通信。**

```yaml
services:
  # 唯一对外入口
  reverse-proxy:
    image: nginx:alpine
    ports:
      - "${LISTEN_PORT:-80}:80"      # 宿主机唯一开放端口

  # 内部服务 — 不写 ports
  app:
    build: .
    # expose 是可选的，仅起文档作用
    # 同一 compose 网络内的容器始终可通过服务名互访
    expose:
      - "8080"

  db:
    image: postgres:17
    # 没有 ports → 外部完全不可达
    volumes:
      - db-data:/var/lib/postgresql/data
```

| 关键字 | 效果 | 用途 |
|--------|------|------|
| `ports: "80:80"` | 映射到宿主机，外部可达 | 仅入口服务 |
| `expose: "8080"` | 仅容器间可见（文档性质） | 内部服务（可选） |
| 都不写 | 同网络容器仍可通过服务名访问 | 内部服务 |

### Dev 环境例外

开发时应用跑在宿主机，需要直连数据库等基础设施：

```yaml
# docker-compose.yml (dev)
services:
  db:
    ports:
      - "${DB_PORT:-5432}:5432"   # 宿主机进程需要连
  redis:
    ports:
      - "${REDIS_PORT:-6379}:6379"
```

如果 dev 也全容器化，则同样不暴露。

---

## 2. 调试 Prod：临时容器，不暴露端口

**永远不要为了调试而给 prod 服务加 `ports`。** 用临时容器接入内部网络：

```bash
# 方式 1：docker compose run（自动加入 compose 网络）
docker compose -f docker-compose.prod.yml run --rm -it --no-deps \
  --entrypoint sh alpine:3.21

# 方式 2：docker run 手动指定网络
docker run --rm -it --network <project-name>_default alpine:3.21 sh
```

在临时容器里：

```bash
apk add curl postgresql-client redis

# 测试服务连通性
curl http://app:8080/health
psql "postgres://user:pass@db:5432/mydb"
redis-cli -h redis ping
```

用完自动删除，零残留、零攻击面。

---

## 3. 健康检查与启动顺序

### 每个长驻服务都必须有 healthcheck

```yaml
services:
  db:
    image: postgres:17
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-postgres}"]
      interval: 5s
      timeout: 5s
      retries: 20

  app:
    build: .
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 5
```

常用 healthcheck 写法：

| 服务 | test 命令 |
|------|-----------|
| PostgreSQL | `pg_isready -U user -d dbname` |
| MySQL | `mysqladmin ping -h localhost` |
| Redis | `redis-cli ping` |
| HTTP 服务 | `wget -q --spider http://localhost:PORT/health` 或 `curl -f http://localhost:PORT/health` |
| 静态进程 | `test -f /tmp/healthy` (进程自己写文件) |

> 注意：alpine 镜像自带 `wget` 但不带 `curl`，优先用 `wget`。

### depends_on 必须带 condition

```yaml
services:
  # 一次性任务（如数据库迁移）
  migrate:
    depends_on:
      db:
        condition: service_healthy            # 等 db 真正能接受连接

  # 长驻服务依赖一次性任务
  app:
    depends_on:
      migrate:
        condition: service_completed_successfully  # 迁移跑完再启动

  # 入口依赖所有上游服务
  reverse-proxy:
    depends_on:
      app:
        condition: service_healthy
```

三种 condition：

| condition | 含义 | 适用场景 |
|-----------|------|----------|
| `service_started` | 容器启动了（默认值，几乎没用） | 不推荐 |
| `service_healthy` | healthcheck 通过 | 长驻服务 |
| `service_completed_successfully` | exit code 0 | 一次性任务（migrate、seed） |

---

## 4. 环境变量

### 必填 vs 可选 vs 允许为空

```yaml
environment:
  DB_PASSWORD: ${DB_PASSWORD:?DB_PASSWORD is required}   # 必填，缺失则启动报错
  LOG_LEVEL: ${LOG_LEVEL:-info}                           # 可选，有默认值
  EXTRA_CORS: ${EXTRA_CORS:-}                             # 可选，允许为空
```

| 语法 | 含义 |
|------|------|
| `${VAR:?msg}` | 未设置或为空 → 报错退出 |
| `${VAR:-default}` | 未设置或为空 → 使用 default |
| `${VAR:-}` | 未设置 → 空字符串 |

### YAML Anchor 复用共享变量

多个服务需要同一组变量时，用 anchor 避免重复：

```yaml
x-db-env: &db-env
  DATABASE_URL: postgres://${DB_USER:-app}:${DB_PASSWORD}@db:5432/${DB_NAME:-app}?sslmode=disable

services:
  migrate:
    environment:
      <<: *db-env

  app:
    environment:
      <<: *db-env
      PORT: "8080"
      SECRET_KEY: ${SECRET_KEY:?required}
```

`x-` 前缀是 compose 的扩展字段，不会被解析为服务。

---

## 5. 数据持久化

### Prod：宿主机路径（可预测、易备份）

```yaml
services:
  db:
    volumes:
      - ${DATA_DIR:-/data/myapp}/postgres:/var/lib/postgresql/data
      
  redis:
    volumes:
      - ${DATA_DIR:-/data/myapp}/redis:/data
```

### Dev：Named Volume（简单、不关心路径）

```yaml
services:
  db:
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

### 备份

```bash
# Prod 数据库备份 — 用临时容器，不需要暴露端口
docker compose exec db pg_dump -U user dbname > backup.sql
```

---

## 6. 重启策略

```yaml
services:
  app:
    restart: unless-stopped     # 崩溃自动重启，手动 stop 后不重启

  db:
    restart: unless-stopped

  migrate:
    # 不设 restart — 一次性任务，跑完退出即可

  reverse-proxy:
    restart: unless-stopped
```

| 策略 | 含义 | 适用 |
|------|------|------|
| `unless-stopped` | 崩溃重启，手动停不重启 | 所有长驻服务 |
| `always` | 任何原因退出都重启 | 极少用 |
| `on-failure` | 非零退出码重启 | 特殊场景 |
| 不设置 | 不重启 | 一次性任务 |

---

## 7. 构建优化

### 多阶段 Dockerfile（最终镜像只含运行时）

**Go 示例：**

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /app ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
```

**Node.js 示例（Next.js standalone）：**

```dockerfile
FROM node:22-alpine AS deps
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
CMD ["node", "server.js"]
```

### .dockerignore

```
.git
node_modules
data/
*.md
.env*
.vscode
```

减小 build context，加快构建速度，防止泄露敏感文件。

---

## 8. 反向代理模式

### Nginx 配置模板

```nginx
upstream app {
    server app:8080;
}

upstream frontend {
    server frontend:3000;
}

server {
    listen 80;
    client_max_body_size 50m;

    # API
    location /api/ {
        proxy_pass http://app;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket — 必须加 Upgrade 头
    location /ws {
        proxy_pass http://app;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;    # 24h，防止长连接被断
        proxy_send_timeout 86400;
    }

    # 前端（兜底）
    location / {
        proxy_pass http://frontend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

`proxy_pass` 使用 Docker 服务名，Docker 内置 DNS 自动解析。

### 挂载配置

```yaml
services:
  reverse-proxy:
    image: nginx:alpine
    volumes:
      - ./deploy/nginx.conf:/etc/nginx/conf.d/default.conf:ro   # :ro 只读
```

---

## 9. 日志

```yaml
services:
  app:
    logging:
      driver: json-file
      options:
        max-size: "10m"     # 单文件最大 10MB
        max-file: "3"       # 最多保留 3 个文件
```

不设限制的话，日志会无限增长直到撑爆磁盘。所有长驻服务都应该加。

---

## 10. 安全

```yaml
services:
  app:
    read_only: true          # 文件系统只读（需要写的目录用 tmpfs）
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges    # 禁止提权

  db:
    read_only: true
    tmpfs:
      - /tmp
      - /run/postgresql
```

其他注意事项：
- **不要把 `.env` 文件提交到 git**，用 `.env.example` 做模板
- Prod 的 secrets 通过 CI/CD 注入（如 GitHub Actions Secrets），不落盘或用完即删
- 容器内进程**不要用 root 运行**（Dockerfile 中 `USER nonroot`）

---

## 11. 文件结构

```
project/
├── docker-compose.yml            # Dev（只有基础设施：db、redis 等）
├── docker-compose.prod.yml       # Prod（完整服务栈）
├── Dockerfile                    # 主应用
├── apps/web/Dockerfile           # 前端（如有）
├── deploy/
│   └── nginx.conf                # 反向代理配置
├── .dockerignore
├── .env.example                  # 环境变量模板（提交到 git）
└── .env                          # 实际值（不提交）
```

### Dev vs Prod 的分法

| | Dev (`docker-compose.yml`) | Prod (`docker-compose.prod.yml`) |
|---|---|---|
| 应用 | 跑在宿主机（热更新） | 跑在容器里 |
| 基础设施 | 容器，暴露端口给宿主机 | 容器，不暴露端口 |
| 数据 | Named volume | 宿主机路径 |
| 反向代理 | 不需要 | Nginx 容器 |

---

## Checklist

新项目配置 Docker Compose 时逐项检查：

- [ ] 只有入口服务（nginx/caddy/traefik）有 `ports`
- [ ] 所有长驻服务有 `healthcheck`
- [ ] 所有 `depends_on` 带 `condition`
- [ ] 必填变量用 `${VAR:?message}` 守护
- [ ] 一次性任务（migrate/seed）不设 `restart`
- [ ] 长驻服务设 `restart: unless-stopped`
- [ ] Dockerfile 使用多阶段构建
- [ ] 有 `.dockerignore`
- [ ] 日志设了 `max-size` 和 `max-file`
- [ ] Prod 数据用宿主机路径持久化
- [ ] `.env` 在 `.gitignore` 中，`.env.example` 提交到 git
