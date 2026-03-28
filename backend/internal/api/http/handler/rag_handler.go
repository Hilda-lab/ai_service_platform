package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	ragservice "ai-service-platform/backend/internal/service/rag"
)

type RAGHandler struct {
	ragService *ragservice.Service
}

func NewRAGHandler(ragService *ragservice.Service) *RAGHandler {
	return &RAGHandler{ragService: ragService}
}

type ingestRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type retrieveRequest struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"top_k"`
}

func (h *RAGHandler) Ingest(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req ingestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "error": err.Error()})
		return
	}

	doc, chunks, err := h.ragService.Ingest(c.Request.Context(), ragservice.IngestRequest{
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"document": doc, "chunks": chunks}})
}

func (h *RAGHandler) ListDocuments(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	docs, err := h.ragService.ListDocuments(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "list documents failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": docs})
}

func (h *RAGHandler) Retrieve(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req retrieveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "error": err.Error()})
		return
	}

	results, err := h.ragService.Retrieve(c.Request.Context(), userID, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}
