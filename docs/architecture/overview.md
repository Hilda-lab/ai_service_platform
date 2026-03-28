# 系统架构概览

## 分层设计
- `api`：Gin 路由、中间件、HTTP Handler
- `service`：业务逻辑（auth/chat/vision/rag/speech/mcp）
- `domain`：实体与仓储接口
- `infrastructure`：MySQL/Redis/RabbitMQ/AI 客户端/向量库/MCP 适配

## 核心链路
1. 用户登录获取 JWT。
2. 前端携带 Token 发起聊天或图像识别请求。
3. 后端根据模型策略选择 OpenAI/Ollama，并支持流式返回。
4. RAG 场景先检索向量库，再拼接上下文给模型。
5. 高并发场景通过 RabbitMQ 异步化非关键路径任务。
