package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/gorilla/websocket"

	chatservice "ai-service-platform/backend/internal/service/chat"
	ragservice "ai-service-platform/backend/internal/service/rag"
)

// Tool 定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Execute     func(context.Context, json.RawMessage) (interface{}, error) `json:"-"`
}

type Hub struct {
	chat  *chatservice.Service
	rag   *ragservice.Service
	tools map[string]*Tool
}

type Client struct {
	userID uint
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
}

type rpcRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID     string      `json:"id,omitempty"`
	Type   string      `json:"type,omitempty"`
	Method string      `json:"method,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type chatSendParams struct {
	SessionID *uint  `json:"session_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Message   string `json:"message"`
	UseRAG    bool   `json:"use_rag"`
}

type chatMessagesParams struct {
	SessionID uint `json:"session_id"`
}

type toolCallParams struct {
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
}

func NewHub(chat *chatservice.Service, rag *ragservice.Service) *Hub {
	h := &Hub{
		chat:  chat,
		rag:   rag,
		tools: make(map[string]*Tool),
	}
	h.registerTools()
	return h
}

// 注册所有可用工具
func (h *Hub) registerTools() {
	// 工具 1: 获取当前时间
	h.tools["get_datetime"] = &Tool{
		Name:        "get_datetime",
		Description: "获取当前日期和时间，返回 RFC3339 格式的时间戳",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "时区，例如 UTC、Asia/Shanghai，默认为 UTC",
				},
			},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var p map[string]interface{}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, fmt.Errorf("invalid params: %v", err)
			}
			return map[string]interface{}{
				"current_time": time.Now().Format(time.RFC3339),
				"unix_timestamp": time.Now().Unix(),
				"timezone": time.Now().Location().String(),
			}, nil
		},
	}

	// 工具 2: 查询 RAG 知识库
	h.tools["query_rag"] = &Tool{
		Name:        "query_rag",
		Description: "查询 RAG 知识库中的文档，基于相似性检索返回相关内容",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "查询关键词或问题",
				},
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "返回最相似的前 K 个结果，默认为 3",
				},
			},
			"required": []string{"query"},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var p struct {
				Query string `json:"query"`
				TopK  int    `json:"top_k"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, fmt.Errorf("invalid params: %v", err)
			}
			if p.TopK == 0 {
				p.TopK = 3
			}
			
			// 调用 RAG 服务检索
			if h.rag == nil {
				return map[string]interface{}{
					"results": []interface{}{},
					"message": "RAG 服务未配置",
				}, nil
			}

			results, err := h.rag.Retrieve(ctx, p.Query, p.TopK)
			if err != nil {
				return nil, fmt.Errorf("rag retrieve error: %v", err)
			}
			return map[string]interface{}{
				"query":   p.Query,
				"results": results,
				"count":   len(results),
			}, nil
		},
	}

	// 工具 3: 获取系统信息
	h.tools["query_system_info"] = &Tool{
		Name:        "query_system_info",
		Description: "获取系统相关信息，如操作系统、Go 版本等",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"fields": map[string]interface{}{
					"type":        "array",
					"description": "要查询的字段：os, arch, go_version, hostname。不指定则返回所有",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var p struct {
				Fields []string `json:"fields"`
			}
			json.Unmarshal(params, &p)

			info := map[string]interface{}{
				"os":        runtime.GOOS,
				"arch":      runtime.GOARCH,
				"go_version": runtime.Version(),
			}

			if hostname, err := os.Hostname(); err == nil {
				info["hostname"] = hostname
			}

			info["current_time"] = time.Now().Format(time.RFC3339)
			return info, nil
		},
	}
}

func (h *Hub) ServeConnection(ctx context.Context, userID uint, conn *websocket.Conn) {
	client := &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 32),
		hub:    h,
	}

	go client.writePump()
	client.sendResponse(rpcResponse{Type: "welcome", Result: map[string]interface{}{"user_id": userID, "ts": time.Now().Unix()}})
	client.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer close(c.send)
	defer c.conn.Close()

	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var req rpcRequest
		if err := json.Unmarshal(message, &req); err != nil {
			c.sendResponse(rpcResponse{Type: "error", Error: "invalid json request"})
			continue
		}

		c.handleRequest(ctx, req)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, req rpcRequest) {
	switch req.Method {
	case "ping":
		c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{"pong": true, "ts": time.Now().Unix()}})
	
	// 工具相关方法
	case "tool.list":
		// 返回可用工具列表
		toolList := make([]map[string]interface{}, 0)
		for _, tool := range c.hub.tools {
			toolList = append(toolList, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			})
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{
			"tools": toolList,
			"count": len(toolList),
		}})
	
	case "tool.call":
		// 执行指定工具
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid params"})
			return
		}

		tool, exists := c.hub.tools[params.ToolName]
		if !exists {
			c.sendResponse(rpcResponse{ID: req.ID, Error: fmt.Sprintf("tool not found: %s", params.ToolName)})
			return
		}

		result, err := tool.Execute(ctx, params.Args)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}

		c.sendResponse(rpcResponse{ID: req.ID, Result: result})
	
	// 聊天相关方法
	case "chat.send":
		var params chatSendParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid params"})
			return
		}

		result, err := c.hub.chat.Complete(ctx, chatservice.ChatRequest{
			UserID:    c.userID,
			SessionID: params.SessionID,
			Provider:  params.Provider,
			Model:     params.Model,
			Message:   params.Message,
			UseRAG:    params.UseRAG,
		})
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}

		c.sendResponse(rpcResponse{ID: req.ID, Result: result})
		c.sendResponse(rpcResponse{Type: "event", Method: "chat.message", Result: result})
	case "chat.sessions":
		sessions, err := c.hub.chat.ListSessions(ctx, c.userID)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: sessions})
	case "chat.messages":
		var params chatMessagesParams
		if err := json.Unmarshal(req.Params, &params); err != nil || params.SessionID == 0 {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid session_id"})
			return
		}
		messages, err := c.hub.chat.ListMessages(ctx, c.userID, params.SessionID)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: messages})
	default:
		c.sendResponse(rpcResponse{ID: req.ID, Error: "unsupported method"})
	}
}

func (c *Client) sendResponse(resp rpcResponse) {
	payload, err := json.Marshal(resp)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
	}
}
