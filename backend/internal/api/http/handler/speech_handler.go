package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	speechservice "ai-service-platform/backend/internal/service/speech"
)

type SpeechHandler struct {
	speech *speechservice.Service
}

func NewSpeechHandler(speech *speechservice.Service) *SpeechHandler {
	return &SpeechHandler{speech: speech}
}

type ttsRequest struct {
	Text     string `json:"text" binding:"required"`
	Model    string `json:"model"`
	Voice    string `json:"voice"`
	Format   string `json:"format"`
	Language string `json:"language"`
}

func (h *SpeechHandler) TextToSpeech(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req ttsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "error": err.Error()})
		return
	}

	result, err := h.speech.Synthesize(c.Request.Context(), speechservice.TTSRequest{
		Text:     req.Text,
		Model:    req.Model,
		Voice:    req.Voice,
		Format:   req.Format,
		Language: req.Language,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *SpeechHandler) SpeechToText(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	fileHeader, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "audio is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "open audio failed"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "read audio failed"})
		return
	}

	text, err := h.speech.Transcribe(c.Request.Context(), speechservice.ASRRequest{
		Model:      strings.TrimSpace(c.PostForm("model")),
		Language:   strings.TrimSpace(c.PostForm("language")),
		Prompt:     strings.TrimSpace(c.PostForm("prompt")),
		FileName:   fileHeader.Filename,
		AudioBytes: data,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"text": text}})
}
