package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"ai-service-platform/backend/internal/api/http/handler"
	"ai-service-platform/backend/internal/api/http/middleware"
)

type Handlers struct {
	Auth *handler.AuthHandler
	Chat *handler.ChatHandler
	RAG  *handler.RAGHandler
	Vision *handler.VisionHandler
}

type RouterOptions struct {
	JWTSecret string
}

func NewRouter(handlers Handlers, opts RouterOptions) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	{
		api.GET("/health", handler.Health)
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Auth.Register)
			auth.POST("/login", handlers.Auth.Login)
			auth.GET("/profile", middleware.JWTAuth(opts.JWTSecret), handlers.Auth.Profile)
		}

		chat := api.Group("/chat", middleware.JWTAuth(opts.JWTSecret))
		{
			chat.GET("/sessions", handlers.Chat.ListSessions)
			chat.GET("/sessions/:id/messages", handlers.Chat.ListMessages)
			chat.POST("/completions", handlers.Chat.Completions)
			chat.POST("/completions/stream", handlers.Chat.CompletionsStream)
		}

		rag := api.Group("/rag", middleware.JWTAuth(opts.JWTSecret))
		{
			rag.POST("/documents", handlers.RAG.Ingest)
			rag.GET("/documents", handlers.RAG.ListDocuments)
			rag.POST("/retrieve", handlers.RAG.Retrieve)
		}

		vision := api.Group("/vision", middleware.JWTAuth(opts.JWTSecret))
		{
			vision.POST("/recognize", handlers.Vision.Recognize)
			vision.POST("/tasks", handlers.Vision.SubmitTask)
			vision.GET("/tasks/:id", handlers.Vision.GetTask)
		}
	}

	return r
}
