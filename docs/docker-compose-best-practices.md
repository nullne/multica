# Docker Compose Production Best Practices

基于 Multica 项目总结的 Docker Compose 生产部署实践。

## 核心原则

**只暴露一个入口端口。** 所有内部服务通过 Docker 内部网络通信，不映射端口到宿主机。

## 网络与端口

### Prod：只暴露 nginx

```yaml
services:
  nginx:
    ports:
      - "${LISTEN_PORT:-80}:80"    # 唯一对外端口

  backend:
    # 没有 ports — 只在 Docker 内部网络可达
    expose:
      - "8080"

  frontend:
    expose:
      - "3000"

  postgres:
    # 没有 ports — 外部完全不可达
```

- `ports` = 映射到宿主机（对外暴露）
- `expose` = 仅容器间可见（可选，主要起文档作用）
- 不写 `ports` 也不写 `expose` = 同一 compose 网络内的容器仍可通过服务名访问

### Dev：按需暴露

Dev 环境下 Go/Next.js 跑在宿主机，需要直连数据库：

```yaml
services:
  postgres:
    ports:
      - "${POSTGRES_PORT:-5432}:5432"   # 宿主机开发进程需要连
```

如果 dev 也全容器化，则同样不暴露。

## 调试 Prod 环境

不要为了调试暴露端口。用临时容器接入内部网络：

```bash
# 起一个临时容器，加入 prod 网络
docker compose -f docker-compose.prod.yml run --rm -it --no-deps \
  --entrypoint sh alpine:3.21

# 安装需要的工具
apk add curl postgresql-client

# 内部网络调试
curl http://backend:8080/health
curl http://frontend:3000
psql "postgres://user:pass@postgres:5432/dbname"
```

用完容器自动删除，零攻击面。

## 服务依赖与启动顺序

```yaml
services:
  migrate:
    depends_on:
      postgres:
        condition: service_healthy     # 等 postgres 真正就绪
    # 一次性任务，跑完就退出

  backend:
    depends_on:
      migrate:
        condition: service_completed_successfully  # 迁移完才启动
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 5

  frontend:
    depends_on:
      backend:
        condition: service_healthy     # backend 就绪才启动

  nginx:
    depends_on:
      - frontend
      - backend
```

关键点：
- **Healthcheck 是必须的**，`depends_on` 的 `condition` 依赖它判断服务就绪
- 一次性任务（如 migrate）用 `service_completed_successfully`
- 长驻服务用 `service_healthy`

## 环境变量

### 必填 vs 可选

```yaml
environment:
  JWT_SECRET: ${JWT_SECRET:?JWT_SECRET is required}   # 缺失则报错退出
  LOG_LEVEL: ${LOG_LEVEL:-info}                        # 缺失则用默认值
  CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-}      # 可为空
```

- `${VAR:?message}` — 强制必填，未设置则 compose 启动直接报错
- `${VAR:-default}` — 有合理默认值的配置
- `${VAR:-}` — 可选，允许为空

### 复用变量

```yaml
x-database-url: &database-url
  DATABASE_URL: postgres://${POSTGRES_USER:-multica}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-multica}?sslmode=disable

services:
  migrate:
    environment:
      <<: *database-url

  backend:
    environment:
      <<: *database-url
      JWT_SECRET: ${JWT_SECRET:?required}
```

用 YAML anchor (`&` / `*`) 避免重复，多个服务共享同一组变量。

## 数据持久化

```yaml
services:
  postgres:
    volumes:
      - ${PGDATA_DIR:-./data/postgres}:/var/lib/postgresql/data
```

- Prod 用**宿主机路径**（`PGDATA_DIR`），数据可预测、易备份
- Dev 可以用 named volume（`pgdata:/var/lib/postgresql/data`），不需要关心路径

## 重启策略

```yaml
services:
  backend:
    restart: unless-stopped    # 崩溃自动重启，手动 stop 后不重启

  migrate:
    # 不设 restart — 一次性任务，跑完就退出
```

## 构建

### 多阶段 Dockerfile

```dockerfile
# Build stage — 包含编译工具，体积大
FROM golang:1.26-alpine AS builder
RUN cd server && CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Runtime stage — 只有二进制和运行时依赖，体积小
FROM alpine:3.21
COPY --from=builder /src/server/bin/server .
```

### Build context 最小化

```yaml
services:
  backend:
    build:
      context: .              # 根目录
      dockerfile: Dockerfile  # 只 COPY 需要的目录
```

配合 `.dockerignore` 排除 `node_modules`、`.git`、`data/` 等。

## Nginx 反向代理模式

```nginx
# API → backend
location /api/ {
    proxy_pass http://backend;
}

# WebSocket — 需要额外的 upgrade 头
location /ws {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;   # 长连接不超时
}

# 其他 → frontend
location / {
    proxy_pass http://frontend;
}
```

所有 `proxy_pass` 使用 Docker 服务名，Docker DNS 自动解析。

## 文件结构

```
project/
├── docker-compose.yml          # Dev（只有基础设施，如 postgres）
├── docker-compose.prod.yml     # Prod（完整服务栈）
├── Dockerfile                  # Backend
├── apps/web/Dockerfile         # Frontend
├── deploy/
│   └── nginx.conf              # Nginx 配置
├── .dockerignore               # 排除不需要的文件
└── .env.example                # 环境变量模板
```

## Checklist

- [ ] 只有 nginx/入口 有 `ports`，其他服务无端口暴露
- [ ] 所有长驻服务有 `healthcheck`
- [ ] `depends_on` 使用 `condition` 而非裸依赖
- [ ] 必填变量用 `${VAR:?message}` 守护
- [ ] 一次性任务（migrate）不设 `restart`
- [ ] Dockerfile 使用多阶段构建
- [ ] 有 `.dockerignore` 文件
- [ ] Prod 数据用宿主机路径持久化
