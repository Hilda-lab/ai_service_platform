package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	mcpservice "ai-service-platform/backend/internal/service/mcp"
	jwtpkg "ai-service-platform/backend/pkg/jwt"
)

type MCPHandler struct {
	hub      *mcpservice.Hub
	jwtSecret string
	upgrader websocket.Upgrader
}

func NewMCPHandler(hub *mcpservice.Hub, jwtSecret string) *MCPHandler {
	return &MCPHandler{
		hub: hub,
		jwtSecret: jwtSecret,
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
	userID, err := h.authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	responseHeader := http.Header{}
	protocolHeader := strings.TrimSpace(c.GetHeader("Sec-WebSocket-Protocol"))
	if protocolHeader != "" {
		parts := strings.Split(protocolHeader, ",")
		if len(parts) > 0 {
			responseHeader.Set("Sec-WebSocket-Protocol", strings.TrimSpace(parts[0]))
		}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "websocket upgrade failed", "error": err.Error()})
		return
	}

	h.hub.ServeConnection(c.Request.Context(), userID, conn)
}

func (h *MCPHandler) authenticate(c *gin.Context) (uint, error) {
	if userIDValue, exists := c.Get("user_id"); exists {
		if userID, ok := userIDValue.(uint); ok && userID > 0 {
			return userID, nil
		}
	}

	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("access_token"))
	}
	if token == "" {
		token = strings.TrimSpace(c.Query("jwt"))
	}
	if token == "" {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[1])
		}
	}
	if token == "" {
		protocolHeader := strings.TrimSpace(c.GetHeader("Sec-WebSocket-Protocol"))
		if protocolHeader != "" {
			items := strings.Split(protocolHeader, ",")
			for _, item := range items {
				candidate := strings.TrimSpace(item)
				if strings.HasPrefix(strings.ToLower(candidate), "bearer ") {
					token = strings.TrimSpace(strings.TrimPrefix(candidate, "Bearer "))
					if token != "" {
						break
					}
				}
			}
			if token == "" && len(items) >= 2 {
				if strings.EqualFold(strings.TrimSpace(items[0]), "bearer") {
					token = strings.TrimSpace(items[1])
				}
			}
		}
	}
	if token == "" {
		if cookie, err := c.Cookie("token"); err == nil {
			token = strings.TrimSpace(cookie)
		}
	}

	if token == "" {
		return 0, http.ErrNoCookie
	}

	claims, err := jwtpkg.ParseToken(token, h.jwtSecret)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}
