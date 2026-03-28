package main

import (
	"context"
	"log"
	"strconv"

	"ai-service-platform/backend/internal/api/http/handler"
	"ai-service-platform/backend/internal/api/http/router"
	"ai-service-platform/backend/internal/config"
	"ai-service-platform/backend/internal/domain/entity"
	ollamaclient "ai-service-platform/backend/internal/infrastructure/ai/ollama"
	openaiclient "ai-service-platform/backend/internal/infrastructure/ai/openai"
	"ai-service-platform/backend/internal/infrastructure/cache/redis"
	"ai-service-platform/backend/internal/infrastructure/db/mysql"
	"ai-service-platform/backend/internal/infrastructure/mq/rabbitmq"
	authservice "ai-service-platform/backend/internal/service/auth"
	chatservice "ai-service-platform/backend/internal/service/chat"
	ragservice "ai-service-platform/backend/internal/service/rag"
	visionservice "ai-service-platform/backend/internal/service/vision"
)

func main() {
	cfg := config.Load()

	db, err := mysql.NewClient(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql init failed: %v", err)
	}

	if err := db.AutoMigrate(&entity.User{}, &entity.ChatSession{}, &entity.ChatMessage{}, &entity.RAGDocument{}, &entity.RAGChunk{}, &entity.VisionTask{}); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	redisClient, err := redis.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("redis init failed: %v", err)
	}

	userRepo := mysql.NewUserRepository(db)
	authSvc := authservice.NewService(userRepo, redisClient, cfg.JWTSecret, cfg.JWTDuration())
	authHandler := handler.NewAuthHandler(authSvc)

	chatRepo := mysql.NewChatRepository(db)
	ragRepo := mysql.NewRAGRepository(db)
	visionRepo := mysql.NewVisionRepository(db)
	ragSvc := ragservice.NewService(ragRepo)
	ragHandler := handler.NewRAGHandler(ragSvc)

	openaiClient := openaiclient.NewClient(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey)
	ollamaClient := ollamaclient.NewClient(cfg.OllamaBaseURL)
	visionSvc := visionservice.NewService(
		visionRepo,
		func(ctx context.Context, model, prompt, mimeType string, imageBytes []byte) (string, error) {
			return openaiClient.AnalyzeImage(ctx, openaiclient.VisionRequest{Model: model, Prompt: prompt, MimeType: mimeType, ImageBytes: imageBytes})
		},
		cfg.RabbitMQURL,
		cfg.VisionQueue,
		cfg.VisionProvider,
		cfg.VisionModel,
		cfg.VisionMock,
	)
	visionHandler := handler.NewVisionHandler(visionSvc)
	chatSvc := chatservice.NewService(chatRepo, redisClient, openaiClient, ollamaClient, ragSvc, cfg.AIProvider, cfg.OpenAIModel, cfg.OllamaModel)
	chatHandler := handler.NewChatHandler(chatSvc)

	if err := rabbitmq.StartVisionConsumer(context.Background(), cfg.RabbitMQURL, cfg.VisionQueue, func(taskID uint) error {
		return visionSvc.ProcessTask(context.Background(), taskID)
	}); err != nil {
		log.Printf("vision consumer disabled: %v", err)
	}

	r := router.NewRouter(
		router.Handlers{Auth: authHandler, Chat: chatHandler, RAG: ragHandler, Vision: visionHandler},
		router.RouterOptions{JWTSecret: cfg.JWTSecret},
	)

	addr := ":" + cfg.HTTPPort
	if _, err := strconv.Atoi(cfg.HTTPPort); err != nil {
		log.Fatalf("invalid HTTP_PORT: %s", cfg.HTTPPort)
	}

	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
