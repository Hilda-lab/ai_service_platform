package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ai-service-platform/backend/internal/domain/entity"
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

	if chunks == nil {
		chunks = []entity.RAGChunk{}
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"document": doc, "chunks": chunks}})
}

func (h *RAGHandler) ListDocuments(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	docs, err := h.ragService.ListDocumentsWithStats(c.Request.Context(), userID)
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

	results, metrics, err := h.ragService.RetrieveWithMetrics(c.Request.Context(), userID, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results, "metrics": metrics})
}

func (h *RAGHandler) DeleteDocument(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "document id is required"})
		return
	}

	// Parse documentID to uint
	var id uint
	if _, err := fmt.Sscanf(documentID, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid document id"})
		return
	}

	// Delete the document (handles both MySQL and Redis)
	if err := h.ragService.DeleteDocument(c.Request.Context(), userID, id); err != nil {
		if err.Error() == "document not found" {
			c.JSON(http.StatusNotFound, gin.H{"message": "document not found"})
			return
		}
		if strings.Contains(err.Error(), "unauthorized") {
			c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "delete document failed", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "document deleted successfully"})
}

func (h *RAGHandler) GetPerformanceStats(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	stats, err := h.ragService.GetPerformanceStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "get stats failed", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
