package mysql

import (
	"context"

	"gorm.io/gorm"

	"ai-service-platform/backend/internal/domain/entity"
)

type RAGRepository struct {
	db *gorm.DB
}

func NewRAGRepository(db *gorm.DB) *RAGRepository {
	return &RAGRepository{db: db}
}

func (r *RAGRepository) CreateDocument(ctx context.Context, document *entity.RAGDocument) error {
	return r.db.WithContext(ctx).Create(document).Error
}

func (r *RAGRepository) CreateChunks(ctx context.Context, chunks []entity.RAGChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&chunks).Error
}

func (r *RAGRepository) ListDocuments(ctx context.Context, userID uint, limit int) ([]entity.RAGDocument, error) {
	var docs []entity.RAGDocument
	query := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *RAGRepository) ListChunks(ctx context.Context, userID uint, limit int) ([]entity.RAGChunk, error) {
	var chunks []entity.RAGChunk
	query := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *RAGRepository) GetChunksByDocumentID(ctx context.Context, documentID uint) ([]entity.RAGChunk, error) {
	var chunks []entity.RAGChunk
	if err := r.db.WithContext(ctx).Where("document_id = ?", documentID).Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *RAGRepository) GetDocumentByID(ctx context.Context, documentID uint) (*entity.RAGDocument, error) {
	var doc entity.RAGDocument
	if err := r.db.WithContext(ctx).Where("id = ?", documentID).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *RAGRepository) DeleteDocument(ctx context.Context, userID uint, documentID uint) error {
	// First, get all chunks for this document to prepare for vector store cleanup
	// Then delete chunks (they have foreign key to document)
	if err := r.db.WithContext(ctx).Where("document_id = ? AND user_id = ?", documentID, userID).Delete(&entity.RAGChunk{}).Error; err != nil {
		return err
	}
	// Finally delete the document itself
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", documentID, userID).Delete(&entity.RAGDocument{}).Error; err != nil {
		return err
	}
	return nil
}
