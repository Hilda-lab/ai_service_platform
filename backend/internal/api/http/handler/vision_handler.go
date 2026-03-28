package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ragrabbit "ai-service-platform/backend/internal/infrastructure/mq/rabbitmq"
	visionservice "ai-service-platform/backend/internal/service/vision"
)

type VisionHandler struct {
	visionService *visionservice.Service
}

func NewVisionHandler(visionService *visionservice.Service) *VisionHandler {
	return &VisionHandler{visionService: visionService}
}

func (h *VisionHandler) Recognize(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	request, err := parseVisionRequest(c, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	task, err := h.visionService.RecognizeSync(c.Request.Context(), request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

func (h *VisionHandler) SubmitTask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	request, err := parseVisionRequest(c, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	task, err := h.visionService.SubmitAsync(c.Request.Context(), request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": task.ID, "status": task.Status}})
}

func (h *VisionHandler) GetTask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	taskID, err := ragrabbit.ParseTaskID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	task, err := h.visionService.GetTask(c.Request.Context(), userID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

func parseVisionRequest(c *gin.Context, userID uint) (visionservice.RecognizeRequest, error) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		return visionservice.RecognizeRequest{}, err
	}

	file, err := fileHeader.Open()
	if err != nil {
		return visionservice.RecognizeRequest{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return visionservice.RecognizeRequest{}, err
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	prompt := strings.TrimSpace(c.PostForm("prompt"))
	provider := strings.TrimSpace(c.PostForm("provider"))
	model := strings.TrimSpace(c.PostForm("model"))

	return visionservice.RecognizeRequest{
		UserID:     userID,
		Provider:   provider,
		Model:      model,
		Prompt:     prompt,
		FileName:   fileHeader.Filename,
		MimeType:   mimeType,
		ImageBytes: data,
	}, nil
}
