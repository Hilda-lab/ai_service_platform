package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"

	chatservice "ai-service-platform/backend/internal/service/chat"
)

type Hub struct {
	chat *chatservice.Service
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

func NewHub(chat *chatservice.Service) *Hub {
	return &Hub{chat: chat}
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
