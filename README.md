# AI 应用服务平台（Go + Gin + Vue）

该项目是一个 AI 应用服务平台骨架，包含：
- Gin 高性能 Web 服务
- AI 聊天（OpenAI / Ollama）
- 图像识别
- JWT 认证与会话管理
- MySQL + Redis
- RabbitMQ 异步消息
- RAG（向量检索增强）
- MCP 服务端（mcp-go）
- 语音识别与 TTS

## 目录结构

- `backend/`：Go 后端服务
- `frontend/`：Vue 前端应用
- `deploy/`：Docker / K8s / Nginx 部署资源
- `docs/`：架构、API 与运行手册

## 快速开始（骨架阶段）

### 1) 启动后端
```bash
cd backend
go mod tidy
go run ./cmd/server
```
默认监听 `:8080`，健康检查：`GET /api/v1/health`。

### 2) 启动前端
```bash
cd frontend
npm install
npm run dev
```

### 3) 使用 Docker 编排（可选）
```bash
cd deploy/docker
docker compose up -d
```

## 下一步建议
1. 完成 `backend/internal/service/*` 业务实现。
2. 落地 MySQL/Redis/RabbitMQ 连接与配置加载。
3. 接入 OpenAI/Ollama 的同步与流式聊天。
4. 增加 RAG 向量检索与知识库管理。
5. 实现 MCP server 与前端会话联动。
