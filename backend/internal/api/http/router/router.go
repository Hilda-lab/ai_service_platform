package router

import (
	"github.com/gin-gonic/gin"

	"ai-service-platform/backend/internal/api/http/handler"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api/v1")
	{
		api.GET("/health", handler.Health)
	}

	return r
}
