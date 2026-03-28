package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	chatservice "ai-service-platform/backend/internal/service/chat"
)

type ChatHandler struct {
	chatService *chatservice.Service
}

func NewChatHandler(chatService *chatservice.Service) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

type chatCompletionRequest struct {
	SessionID *uint  `json:"session_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Message   string `json:"message" binding:"required"`
	UseRAG    bool   `json:"use_rag"`
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	sessions, err := h.chatService.ListSessions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "list sessions failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	sessionID64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
		return
	}

	messages, err := h.chatService.ListMessages(c.Request.Context(), userID, uint(sessionID64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": messages})
}

func (h *ChatHandler) Completions(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "error": err.Error()})
		return
	}

	result, err := h.chatService.Complete(c.Request.Context(), chatservice.ChatRequest{
		UserID:    userID,
		SessionID: req.SessionID,
		Provider:  req.Provider,
		Model:     req.Model,
		Message:   req.Message,
		UseRAG:    req.UseRAG,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *ChatHandler) CompletionsStream(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	result, err := h.chatService.Stream(c.Request.Context(), chatservice.ChatRequest{
		UserID:    userID,
		SessionID: req.SessionID,
		Provider:  req.Provider,
		Model:     req.Model,
		Message:   req.Message,
		UseRAG:    req.UseRAG,
	}, func(chunk string) error {
		payload, _ := json.Marshal(gin.H{"type": "chunk", "content": chunk})
		if _, writeErr := c.Writer.Write(append([]byte("data: "), append(payload, []byte("\n\n")...)...)); writeErr != nil {
			return writeErr
		}
		c.Writer.Flush()
		return nil
	})
	if err != nil {
		payload, _ := json.Marshal(gin.H{"type": "error", "message": err.Error()})
		_, _ = c.Writer.Write(append([]byte("data: "), append(payload, []byte("\n\n")...)...))
		c.Writer.Flush()
		return
	}

	donePayload, _ := json.Marshal(gin.H{
		"type":       "done",
		"session_id": result.SessionID,
		"provider":   result.Provider,
		"model":      result.Model,
	})
	_, _ = c.Writer.Write(append([]byte("data: "), append(donePayload, []byte("\n\n")...)...))
	c.Writer.Flush()
}

func getUserIDFromContext(c *gin.Context) (uint, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	userID, ok := value.(uint)
	if !ok {
		return 0, false
	}
	return userID, true
}
