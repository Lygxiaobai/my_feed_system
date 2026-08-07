# my_feed_system

> 一个面向短视频场景的 Feed 流系统：Go API + 异步 Worker + Vue 3 Web 应用。

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-42B883?logo=vue.js)](https://vuejs.org/)
[![Docker Compose](https://img.shields.io/badge/Docker%20Compose-ready-2496ED?logo=docker)](https://docs.docker.com/compose/)

## 项目亮点

- 多种 Feed：最新流、关注流、点赞排行和热度排行。
- 异步写入：RabbitMQ Worker 处理点赞、评论、社交、时间线和热度任务。
- 一致性设计：Outbox、幂等、消息去重和死信处理降低异步失败风险。
- 分层缓存：Redis 配合本地缓存加速 Feed、视频详情和热度数据。
- 可观测性：提供指标和 pprof 入口，便于定位运行时问题。
- 可复现启动：Docker Compose 一次启动 API、Worker、前端和基础设施。

## 系统结构

```mermaid
flowchart LR
    Browser[Browser] --> Frontend[Vue 3 + Nginx]
    Frontend --> API[Go HTTP API]
    API --> MySQL[(MySQL)]
    API --> Redis[(Redis)]
    API --> Outbox[Outbox]
    Outbox --> RabbitMQ[(RabbitMQ)]
    RabbitMQ --> Worker[Go Worker]
    Worker --> MySQL
    Worker --> Redis
```

API 负责同步请求和公开 HTTP 接口，Worker 独立消费异步任务。两者共享配置和基础设施，但保持进程职责分离。

## 功能范围

| 模块 | 能力 |
| --- | --- |
| 账号 | 注册、登录、登出、改名、改密、资料查询 |
| 视频 | 上传、发布、详情、作者视频、点赞视频 |
| Feed | 最新流、关注流、点赞排行、热度排行、游标分页 |
| 互动 | 点赞、取消点赞、评论、回复、删除、关注、取关 |
| 工程能力 | Redis 缓存、RabbitMQ Worker、Outbox、幂等、限流、指标、pprof |

## 快速启动

### 环境要求

- Docker Desktop 或 Docker Engine
- Docker Compose

### Compose 启动

```bash
git clone git@github.com:Lygxiaobai/my_feed_sytem.git
cd my_feed_sytem
docker compose up -d --build
```

启动后访问：

| 服务 | 地址 |
| --- | --- |
| Web 前端 | <http://localhost:5173> |
| API 健康检查 | <http://localhost:8080/ping> |
| RabbitMQ 管理台 | <http://localhost:15672> |

Compose 中的数据库和 RabbitMQ 凭据仅用于本地开发。部署到公网前必须替换默认配置，并将敏感配置放在仓库外部。

停止服务：

```bash
docker compose down
```

同时删除本地数据卷：

```bash
docker compose down -v
```

> `down -v` 会删除 MySQL、Redis、RabbitMQ 和上传文件的本地持久化数据，请确认后再执行。

## 本地开发

### 后端 API

```powershell
cd .\backend
go run ./cmd
```

### 异步 Worker

```powershell
cd .\backend
go run ./cmd/worker
```

### 前端

```powershell
cd .\frontend
npm install
npm run dev
```

前端开发服务器默认运行在 `127.0.0.1:5173`，通过 Vite 代理访问本地 API。后端默认开发端口为 `127.0.0.1:8081`，代理规则见 [`frontend/vite.config.ts`](./frontend/vite.config.ts)。

## 配置与部署边界

- Compose 使用 [`backend/configs/config.docker.yaml`](./backend/configs/config.docker.yaml) 作为容器配置模板。
- 本地手动运行使用 [`backend/configs/config.yaml`](./backend/configs/config.yaml)。
- 数据库密码、Redis 密码、RabbitMQ 密码和 JWT 密钥不得提交到仓库。
- Cloudflare、域名和外部反向代理属于部署环境配置，不属于本项目代码契约。

## 项目文档

- [`AGENTS.md`](./AGENTS.md)：Agent 开发流程和项目级约束。
- [`.spec/`](./.spec/)：业务行为、架构边界和验证场景。
- [`my_feed_system系统设计说明.md`](./my_feed_system系统设计说明.md)：系统设计说明。
- [`docker-compose.yaml`](./docker-compose.yaml)：本地 Compose 启动编排。

## 仓库结构

```text
backend/      Go API、Worker 和业务模块
frontend/     Vue 3 + Vite Web 应用
nginx/        本地代理和开发辅助脚本
.spec/        行为与架构契约
AGENTS.md     Agent 工作规则
```

## 验证

```bash
# 前端类型检查与生产构建
cd frontend
npm run build

# 后端测试
cd ../backend
go test ./...
```

项目的行为场景和架构约束以 [`.spec/`](./.spec/) 为准；实现变更后应同步检查对应的 `spec.md` 和 `eval.md`。
