package repository

import (
	"context"

	"ai-service-platform/backend/internal/domain/entity"
)

type ChatRepository interface {
	CreateSession(ctx context.Context, session *entity.ChatSession) error
	GetSessionByID(ctx context.Context, sessionID, userID uint) (*entity.ChatSession, error)
	ListSessions(ctx context.Context, userID uint, limit int) ([]entity.ChatSession, error)
	CreateMessage(ctx context.Context, message *entity.ChatMessage) error
	ListMessages(ctx context.Context, sessionID, userID uint, limit int) ([]entity.ChatMessage, error)
}
