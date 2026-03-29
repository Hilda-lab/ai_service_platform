# MCP 工具使用指南

MCP（Model Context Protocol）现在提供 3 个实用工具，可以通过 WebSocket 调用。

---

## 快速开始

### 1. 安装 wscat（WebSocket 客户端）

```bash
npm install -g wscat
```

### 2. 连接 MCP WebSocket

```bash

# 先登录拿到 JWT（示例）
curl -s -X POST http://127.0.0.1:28080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"testuser@example.com","password":"TestPassword123"}' 
# 使用 query 参数传 token（推荐）
wscat -c "ws://127.0.0.1:28080/api/v1/mcp/ws?token=$TOKEN"

# 也可使用 Header 传 token
wscat -c ws://127.0.0.1:28080/api/v1/mcp/ws -H "Authorization: Bearer $TOKEN"
```

> 说明：MCP WebSocket 需要鉴权，不带 token 会返回 401。

---

## 可用工具

### 工具 1: `get_datetime` - 获取当前日期时间

**用途：** 获取实时时间戳（解决 AI 不知道当前时间的问题）

**使用方法：**

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "tool.call",
  "params": {
    "tool_name": "get_datetime",
    "args": {
      "timezone": "Asia/Shanghai"
    }
  }
}
```

**预期响应：**

```json
{
  "result": {
    "current_time": "2026-03-29T14:33:45+08:00",
    "unix_timestamp": 1743320025,
    "timezone": "Asia/Shanghai"
  }
}
```

**AI 场景：** "现在几点了？" → AI 调用此工具获取实时时间 → 返回准确答案

---

### 工具 2: `query_rag` - 查询知识库

**用途：** 检索 RAG 知识库中的相关文档（必须先上传文档）

**使用方法：**

```json
{
  "jsonrpc": "2.0",
  "id": "2",
  "method": "tool.call",
  "params": {
    "tool_name": "query_rag",
    "args": {
      "query": "产品价格是多少",
      "top_k": 3
    }
  }
}
```

**预期响应：**

```json
{
  "result": {
    "query": "产品价格是多少",
    "results": [
      {
        "content": "标准版本价格为 999 元...",
        "score": 0.92,
        "source": "pricing.pdf"
      },
      {
        "content": "企业版本价格为 2999 元...",
        "score": 0.88,
        "source": "pricing.pdf"
      }
    ],
    "count": 2
  }
}
```

**AI 场景：** "我们的产品价格?" → AI 调用此工具 → 从知识库检索 → 返回准确的产品定价

---

### 工具 3: `query_system_info` - 获取系统信息

**用途：** 获取系统配置信息（OS、架构、Go 版本等）

**使用方法：**

```json
{
  "jsonrpc": "2.0",
  "id": "3",
  "method": "tool.call",
  "params": {
    "tool_name": "query_system_info",
    "args": {
      "fields": ["os", "go_version"]
    }
  }
}
```

**预期响应：**

```json
{
  "result": {
    "os": "windows",
    "arch": "amd64",
    "go_version": "go1.23.0",
    "hostname": "DESKTOP-ABC",
    "current_time": "2026-03-29T14:33:45+08:00"
  }
}
```

---

## 查看所有可用工具

**获取工具列表：**

```json
{
  "jsonrpc": "2.0",
  "id": "tools",
  "method": "tool.list",
  "params": {}
}
```

**响应示例：**

```json
{
  "result": {
    "tools": [
      {
        "name": "get_datetime",
        "description": "获取当前日期和时间，返回 RFC3339 格式的时间戳",
        "parameters": { ... }
      },
      {
        "name": "query_rag",
        "description": "查询 RAG 知识库中的文档...",
        "parameters": { ... }
      },
      {
        "name": "query_system_info",
        "description": "获取系统相关信息...",
        "parameters": { ... }
      }
    ],
    "count": 3
  }
}
```

---

## 完整测试流程

### 第 1 步：连接 WebSocket

```bash
# 先登录获取 token
TOKEN=$(curl -s -X POST http://127.0.0.1:28080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"testuser@example.com","password":"TestPassword123"}' | jq -r '.data.token')

# 带 token 连接
wscat -c "ws://127.0.0.1:28080/api/v1/mcp/ws?token=$TOKEN"
```

### 第 2 步：查看可用工具

粘贴并发送：

```json
{"jsonrpc":"2.0","id":"tools","method":"tool.list","params":{}}
```

### 第 3 步：测试 get_datetime 工具

```json
{"jsonrpc":"2.0","id":"1","method":"tool.call","params":{"tool_name":"get_datetime","args":{}}}
```

### 第 4 步：测试 query_system_info 工具

```json
{"jsonrpc":"2.0","id":"2","method":"tool.call","params":{"tool_name":"query_system_info","args":{}}}
```

### 第 5 步：测试 query_rag 工具（如果已上传文档）

```json
{"jsonrpc":"2.0","id":"3","method":"tool.call","params":{"tool_name":"query_rag","args":{"query":"测试查询","top_k":3}}}
```

---

## MCP 工具的真实价值

现在 AI 可以：

| 场景 | 之前 | 现在 |
|------|------|------|
| 用户问现在几点 | ❌ "我不知道" | ✅ 实时返回当前时间 |
| 用户问产品信息 | ❌ "我没有这些信息" | ✅ 从知识库检索准确答案 |
| 用户问系统配置 | ❌ "我无法获取" | ✅ 返回实时系统信息 |

---

## 下一步扩展（可选）

你可以继续添加更多工具，例如：

```go
// 工具 4: 执行数据库查询
h.tools["query_database"] = &Tool{...}

// 工具 5: 管理 TODO 任务
h.tools["create_todo"] = &Tool{...}

// 工具 6: 发送邮件/通知
h.tools["send_email"] = &Tool{...}

// 工具 7: 调用外部 API
h.tools["call_api"] = &Tool{...}
```

---

## 调试技巧

- **WebSocket 连接 401？** 通常是未携带 JWT。请先登录拿 token，并通过 `?token=` 或 `Authorization: Bearer` 传入
- **WebSocket 连接失败？** 确保后端在 28080 端口运行，且 MCP 路由是 `/api/v1/mcp/ws`
- **工具返回错误？** 检查 `args` 参数是否与工具定义的 `parameters` 匹配
- **知识库查询无结果？** 先上传文档到 RAG，确保已经有向量化的文档

---

## 总结

✅ MCP 现在支持 3 个工具，使 AI 能：
- 获取实时信息
- 查询知识库
- 获取系统状态

这就是 **AI 真正的价值** - 不仅仅回答问题，还能**获取信息**并**执行操作**！

