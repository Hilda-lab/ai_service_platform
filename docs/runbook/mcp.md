# MCP 网关使用说明

当前阶段已提供 MCP 风格实时网关（WebSocket + JSON-RPC 风格消息），用于多客户端并发连接和实时消息交互。

## 连接地址
- `GET /api/v1/mcp/ws`
- 需要 Header: `Authorization: Bearer <JWT_TOKEN>`

## 消息格式
客户端请求：
```json
{
  "id": "req-1",
  "method": "chat.send",
  "params": {
    "session_id": 1,
    "provider": "openai",
    "model": "gpt-5.1",
    "message": "你好",
    "use_rag": true
  }
}
```

服务端响应：
```json
{
  "id": "req-1",
  "result": {
    "session_id": 1,
    "provider": "openai",
    "model": "gpt-5.1",
    "reply": "你好！"
  }
}
```

## 支持方法
- `ping`
- `chat.send`
- `chat.sessions`
- `chat.messages`

## 事件消息
- 连接成功后会收到 `type=welcome`
- 聊天完成后会收到 `type=event, method=chat.message`
