package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ai-service-platform/backend/internal/domain/entity"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateSession(ctx context.Context, session *entity.ChatSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *ChatRepository) GetSessionByID(ctx context.Context, sessionID, userID uint) (*entity.ChatSession, error) {
	var session entity.ChatSession
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *ChatRepository) ListSessions(ctx context.Context, userID uint, limit int) ([]entity.ChatSession, error) {
	var sessions []entity.ChatSession
	query := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *ChatRepository) CreateMessage(ctx context.Context, message *entity.ChatMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *ChatRepository) ListMessages(ctx context.Context, sessionID, userID uint, limit int) ([]entity.ChatMessage, error) {
	var messages []entity.ChatMessage
	query := r.db.WithContext(ctx).
		Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}
