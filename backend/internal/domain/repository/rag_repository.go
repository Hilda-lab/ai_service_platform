package repository

import (
	"context"

	"ai-service-platform/backend/internal/domain/entity"
)

type RAGRepository interface {
	CreateDocument(ctx context.Context, document *entity.RAGDocument) error
	CreateChunks(ctx context.Context, chunks []entity.RAGChunk) error
	ListDocuments(ctx context.Context, userID uint, limit int) ([]entity.RAGDocument, error)
	ListChunks(ctx context.Context, userID uint, limit int) ([]entity.RAGChunk, error)
	GetChunksByDocumentID(ctx context.Context, documentID uint) ([]entity.RAGChunk, error)
}
