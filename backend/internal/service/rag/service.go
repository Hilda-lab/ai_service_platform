package rag

import (
	"context"
	"errors"
	"sort"
	"strings"

	"ai-service-platform/backend/internal/domain/entity"
	"ai-service-platform/backend/internal/domain/repository"
	"ai-service-platform/backend/internal/infrastructure/rag/vectordb"
)

type Service struct {
	repo repository.RAGRepository
}

type IngestRequest struct {
	UserID  uint
	Title   string
	Content string
}

type RetrieveResult struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func NewService(repo repository.RAGRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Ingest(ctx context.Context, req IngestRequest) (*entity.RAGDocument, int, error) {
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if title == "" || content == "" {
		return nil, 0, errors.New("title and content are required")
	}

	doc := &entity.RAGDocument{
		UserID:  req.UserID,
		Title:   title,
		Content: content,
	}
	if err := s.repo.CreateDocument(ctx, doc); err != nil {
		return nil, 0, err
	}

	parts := splitContent(content, 400)
	chunks := make([]entity.RAGChunk, 0, len(parts))
	for _, part := range parts {
		vector := vectordb.Embed(part, 128)
		embedding, err := vectordb.MarshalVector(vector)
		if err != nil {
			return nil, 0, err
		}
		chunks = append(chunks, entity.RAGChunk{
			DocumentID: doc.ID,
			UserID:     req.UserID,
			Content:    part,
			Embedding:  embedding,
		})
	}

	if err := s.repo.CreateChunks(ctx, chunks); err != nil {
		return nil, 0, err
	}

	return doc, len(chunks), nil
}

func (s *Service) ListDocuments(ctx context.Context, userID uint) ([]entity.RAGDocument, error) {
	return s.repo.ListDocuments(ctx, userID, 100)
}

func (s *Service) Retrieve(ctx context.Context, userID uint, query string, topK int) ([]RetrieveResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query is required")
	}
	if topK <= 0 {
		topK = 3
	}

	chunks, err := s.repo.ListChunks(ctx, userID, 1000)
	if err != nil {
		return nil, err
	}

	queryVector := vectordb.Embed(query, 128)
	results := make([]RetrieveResult, 0, len(chunks))
	for _, chunk := range chunks {
		vector, unmarshalErr := vectordb.UnmarshalVector(chunk.Embedding)
		if unmarshalErr != nil {
			continue
		}
		score := cosine(queryVector, vector)
		if score <= 0 {
			continue
		}
		results = append(results, RetrieveResult{Content: chunk.Content, Score: score})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *Service) RetrieveContents(ctx context.Context, userID uint, query string, topK int) ([]string, error) {
	results, err := s.Retrieve(ctx, userID, query, topK)
	if err != nil {
		return nil, err
	}
	contents := make([]string, 0, len(results))
	for _, item := range results {
		contents = append(contents, item.Content)
	}
	return contents, nil
}

func splitContent(content string, maxRunes int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	runes := []rune(content)
	if len(runes) <= maxRunes {
		return []string{content}
	}

	chunks := make([]string, 0)
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[start:end])))
	}
	return chunks
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	size := len(a)
	if len(b) < size {
		size = len(b)
	}
	var dot float64
	for i := 0; i < size; i++ {
		dot += a[i] * b[i]
	}
	return dot
}
