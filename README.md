# AI 服务平台

一个基于 Go + Gin + Vue 的 AI 应用服务平台，支持认证、聊天、RAG、语音、视觉识别与 MCP WebSocket 能力。

## 核心能力

- 用户注册、登录、JWT 鉴权
- Chat 会话与消息管理
- Chat + RAG 检索增强问答
- 多模型接入（OpenAI / Ollama）
- 文档摄入与向量检索（RAG）
- 语音能力（TTS / ASR）
- 图像识别（同步与异步任务）
- MCP WebSocket 通道

## 项目结构

- backend: Go 后端服务
- frontend: Vue 前端应用
- deploy: Docker / K8s / Nginx 部署资源
- docs: 架构、API 与运行手册

## 技术栈

- 后端: Go, Gin, GORM
- 前端: Vue 3, Vite, TypeScript
- 数据与中间件: MySQL, Redis, RabbitMQ
- AI 能力: OpenAI Relay API / Ollama

## 环境要求

- Go 1.22+
- Node.js 18+
- MySQL 8+
- Redis 6+
- RabbitMQ 3.12+（视觉异步任务需要）

## 快速开始

### 1. 启动依赖服务（推荐）

```bash
cd deploy/docker
docker compose up -d
```

### 2. 启动后端

```bash
cd backend
go mod tidy
go run ./cmd/server
```

后端默认监听: 8080  
健康检查接口: GET /api/v1/health

### 3. 启动前端

```bash
cd frontend
npm install
npm run dev
```

前端默认开发地址通常为: http://localhost:5173

## 后端配置

配置由环境变量或配置文件加载，示例见 backend/internal/config/config.example.yaml。

常用配置项（按功能分组）：

- 服务
	- HTTP_PORT
	- JWT_SECRET
	- JWT_EXPIRE_HOURS
- MySQL
	- MYSQL_DSN
- Redis
	- REDIS_ADDR
	- REDIS_PASSWORD
	- REDIS_DB
- RabbitMQ
	- RABBITMQ_URL
	- VISION_QUEUE
- AI Provider
	- AI_PROVIDER
	- OPENAI_BASE_URL
	- OPENAI_API_KEY
	- OPENAI_MODEL
	- OLLAMA_BASE_URL
	- OLLAMA_MODEL
- 语音与视觉
	- SPEECH_PROVIDER
	- SPEECH_TTS_MODEL
	- SPEECH_ASR_MODEL
	- SPEECH_VOICE
	- SPEECH_LANGUAGE
	- SPEECH_MOCK
	- VISION_PROVIDER
	- VISION_MODEL
	- VISION_MOCK

## API 概览

- Auth
	- POST /api/v1/auth/register
	- POST /api/v1/auth/login
	- GET /api/v1/auth/profile
- Chat
	- GET /api/v1/chat/sessions
	- GET /api/v1/chat/sessions/:id/messages
	- POST /api/v1/chat/completions
	- POST /api/v1/chat/completions/stream
- RAG
	- POST /api/v1/rag/documents
	- GET /api/v1/rag/documents
	- POST /api/v1/rag/retrieve
- Vision
	- POST /api/v1/vision/recognize
	- POST /api/v1/vision/tasks
	- GET /api/v1/vision/tasks/:id
- Speech
	- POST /api/v1/speech/tts
	- POST /api/v1/speech/asr
- MCP
	- GET /api/v1/mcp/ws

## 开发与验证

在 backend 目录执行：

```bash
go test ./...
```

如果只验证入口构建：

```bash
go test ./cmd/server
```

## 常见问题

- 后端启动失败: 优先检查 MYSQL_DSN、JWT_SECRET、OPENAI_API_KEY 是否正确。
- 前端跨域报错: 确认前端地址为 localhost:5173 或 localhost:5174（后端已配置 CORS 白名单）。
- 聊天无回复或异常: 检查 OPENAI_BASE_URL、OPENAI_API_KEY、AI_PROVIDER 与模型配置是否匹配。
- 视觉异步任务不消费: 检查 RabbitMQ 是否可连接，以及 VISION_QUEUE 配置是否一致。

## 文档索引

- API 定义: docs/api/openapi.yaml
- 架构说明: docs/architecture/overview.md
- 运维手册: docs/runbook/mcp.md, docs/runbook/relay-api.md
- 测试说明: TESTGUIDE.md
- 功能验证报告: VERIFICATION_REPORT.md

## 说明

本仓库为可持续迭代的工程项目。若你在二次开发时调整了路由、配置项或模块初始化，请同步更新本文档，确保前后端与运维信息保持一致。
