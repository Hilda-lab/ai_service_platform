# 中转 API 接入说明（12API）

## 1. 推荐接入方式
- OpenAI 兼容：`POST /v1/chat/completions`
- Base URL：`https://cdn.12ai.org`
- 认证：`Authorization: Bearer <YOUR_API_KEY>`

## 2. 需要修改的配置
在 `backend/.env`（可从 `.env.example` 复制）中设置：

```dotenv
AI_PROVIDER=openai
OPENAI_API_KEY=你的12API密钥
OPENAI_BASE_URL=https://cdn.12ai.org
OPENAI_MODEL=gpt-5.1

# 如需 Gemini 格式
GEMINI_API_KEY=你的12API密钥
GEMINI_BASE_URL=https://cdn.12ai.org
GEMINI_MODEL=gemini-3-pro-preview
```

## 3. 代码位置
- 配置读取：`backend/internal/config/config.go`
- OpenAI 兼容客户端：`backend/internal/infrastructure/ai/openai/client.go`

## 4. 典型调用
- 非流式：`ChatCompletions`
- 流式：`ChatCompletionsStream`

## 5. 说明
- 你当前项目已具备认证主链路（注册/登录/JWT），AI 聊天业务可在 `internal/service/chat` 中注入该客户端并实现路由。
- 若改为 Gemini 原生格式，可在 `internal/infrastructure/ai/gemini` 新增客户端，按 `?key=<YOUR_API_KEY>` 方式鉴权。
