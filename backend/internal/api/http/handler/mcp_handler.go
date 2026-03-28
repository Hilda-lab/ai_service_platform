package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	mcpservice "ai-service-platform/backend/internal/service/mcp"
)

type MCPHandler struct {
	hub      *mcpservice.Hub
	upgrader websocket.Upgrader
}

func NewMCPHandler(hub *mcpservice.Hub) *MCPHandler {
	return &MCPHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" || origin == ""
			},
		},
	}
}

func (h *MCPHandler) WebSocket(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "websocket upgrade failed", "error": err.Error()})
		return
	}

	h.hub.ServeConnection(c.Request.Context(), userID, conn)
}
