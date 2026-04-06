package rag

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"ai-service-platform/backend/internal/domain/entity"
	"ai-service-platform/backend/internal/domain/repository"
	"ai-service-platform/backend/internal/infrastructure/cache/redis"
	"ai-service-platform/backend/internal/infrastructure/rag/vectordb"
	goredis "github.com/redis/go-redis/v9"
)

type Service struct {
	repo          repository.RAGRepository
	vectorStore   *redis.VectorStore
	
	// 性能统计
	statsMu              sync.RWMutex
	totalIngestCalls     int64
	totalSearchCalls     int64
	totalIngestTimeMs    int64
	totalSearchTimeMs    int64
	lastIngestTimeMs     int64
	lastSearchTimeMs     int64
}

type IngestRequest struct {
	UserID  uint
	Title   string
	Content string
}

type DocumentWithStats struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	ChunkCount int    `json:"chunk_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type RetrieveResult struct {
	ChunkID    uint    `json:"chunk_id"`
	DocumentID uint    `json:"document_id"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

type RetrieveMetrics struct {
	Query         string  `json:"query"`
	TopK          int     `json:"top_k"`
	CorpusSize    int     `json:"corpus_size"`
	MatchedCount  int     `json:"matched_count"`
	MaxScore      float64 `json:"max_score"`
	AverageScore  float64 `json:"average_score"`
	MinScoreLimit float64 `json:"min_score_limit"`
	DurationMS    int64   `json:"duration_ms"` // 搜索耗时（毫秒）
}

type PerformanceStats struct {
	TotalDocuments    int     `json:"total_documents"`
	TotalChunks       int     `json:"total_chunks"`
	AvgIngestTimeMs   float64 `json:"avg_ingest_time_ms"`
	AvgSearchTimeMs   float64 `json:"avg_search_time_ms"`
	TotalIngestCalls  int64   `json:"total_ingest_calls"`
	TotalSearchCalls  int64   `json:"total_search_calls"`
	LastIngestTimeMs  int64   `json:"last_ingest_time_ms"`
	LastSearchTimeMs  int64   `json:"last_search_time_ms"`
}

const (
	embedDims       = 128
	defaultChunkLen = 320
	chunkOverlapLen = 64
	minScoreLimit   = 0.08
	fallbackScoreLimit = 0.03
)

func NewService(repo repository.RAGRepository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithRedis(repo repository.RAGRepository, redisClient *goredis.Client) *Service {
	return &Service{
		repo:        repo,
		vectorStore: redis.NewVectorStore(redisClient),
	}
}

func (s *Service) Ingest(ctx context.Context, req IngestRequest) (*entity.RAGDocument, []entity.RAGChunk, error) {
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime).Milliseconds()
		s.statsMu.Lock()
		s.totalIngestCalls++
		s.totalIngestTimeMs += elapsed
		s.lastIngestTimeMs = elapsed
		s.statsMu.Unlock()
	}()
	
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if title == "" || content == "" {
		return nil, nil, errors.New("title and content are required")
	}

	doc := &entity.RAGDocument{
		UserID:  req.UserID,
		Title:   title,
		Content: content,
	}
	if err := s.repo.CreateDocument(ctx, doc); err != nil {
		return nil, nil, err
	}

	parts := splitContent(content, defaultChunkLen, chunkOverlapLen)
	chunks := make([]entity.RAGChunk, 0, len(parts))
	for _, part := range parts {
		vector := vectordb.Embed(part, embedDims)
		embedding, err := vectordb.MarshalVector(vector)
		if err != nil {
			return nil, nil, err
		}
		chunks = append(chunks, entity.RAGChunk{
			DocumentID: doc.ID,
			UserID:     req.UserID,
			Content:    part,
			Embedding:  embedding,
		})
	}

	if err := s.repo.CreateChunks(ctx, chunks); err != nil {
		return nil, nil, err
	}

	// 如果配置了 Redis，批量存储向量到 Redis（使用 Pipeline 优化性能）
	if s.vectorStore != nil && len(chunks) > 0 {
		// 准备批量数据
		chunkDataList := make([]redis.ChunkData, 0, len(chunks))
		for _, chunk := range chunks {
			vector, err := vectordb.UnmarshalVector(chunk.Embedding)
			if err != nil {
				// 如果反序列化失败，继续
				continue
			}
			chunkDataList = append(chunkDataList, redis.ChunkData{
				ChunkID:    chunk.ID,
				DocumentID: chunk.DocumentID,
				Content:    chunk.Content,
				Vector:     vector,
			})
		}
		// 使用 Pipeline 批量存储，忽略错误（MySQL 中已有数据）
		_ = s.vectorStore.StoreChunksBatch(ctx, req.UserID, chunkDataList)
	}

	return doc, chunks, nil
}

func (s *Service) ListDocuments(ctx context.Context, userID uint) ([]entity.RAGDocument, error) {
	return s.repo.ListDocuments(ctx, userID, 100)
}

func (s *Service) ListDocumentsWithStats(ctx context.Context, userID uint) ([]DocumentWithStats, error) {
	docs, err := s.repo.ListDocuments(ctx, userID, 100)
	if err != nil {
		return nil, err
	}

	result := make([]DocumentWithStats, 0, len(docs))
	for _, doc := range docs {
		chunks, err := s.repo.GetChunksByDocumentID(ctx, doc.ID)
		if err != nil {
			// 如果获取chunk失败，假设为0
			chunks = []entity.RAGChunk{}
		}
		result = append(result, DocumentWithStats{
			ID:         doc.ID,
			UserID:     doc.UserID,
			Title:      doc.Title,
			Content:    doc.Content,
			ChunkCount: len(chunks),
			CreatedAt:  doc.CreatedAt.String(),
			UpdatedAt:  doc.UpdatedAt.String(),
		})
	}

	return result, nil
}

func (s *Service) Retrieve(ctx context.Context, userID uint, query string, topK int) ([]RetrieveResult, error) {
	results, _, err := s.RetrieveWithMetrics(ctx, userID, query, topK)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Service) RetrieveWithMetrics(ctx context.Context, userID uint, query string, topK int) ([]RetrieveResult, RetrieveMetrics, error) {
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime).Milliseconds()
		s.statsMu.Lock()
		s.totalSearchCalls++
		s.totalSearchTimeMs += elapsed
		s.lastSearchTimeMs = elapsed
		s.statsMu.Unlock()
	}()
	
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, RetrieveMetrics{}, errors.New("query is required")
	}
	if topK <= 0 {
		topK = 3
	}

	// 优先使用 Redis 进行检索（更高效）
	if s.vectorStore != nil {
		results, metrics, err := s.retrieveFromRedis(ctx, userID, query, topK)
		metrics.DurationMS = time.Since(startTime).Milliseconds()
		return results, metrics, err
	}

	// 回退到 MySQL 检索
	results, metrics, err := s.retrieveFromMySQL(ctx, userID, query, topK)
	metrics.DurationMS = time.Since(startTime).Milliseconds()
	return results, metrics, err
}

// retrieveFromRedis 从 Redis 检索相关文档
func (s *Service) retrieveFromRedis(ctx context.Context, userID uint, query string, topK int) ([]RetrieveResult, RetrieveMetrics, error) {
	// 获取用户所有分块ID
	chunkIDs, err := s.vectorStore.ListUserChunks(ctx, userID)
	if err != nil {
		// 如果 Redis 操作失败，回退到 MySQL
		return s.retrieveFromMySQL(ctx, userID, query, topK)
	}

	if len(chunkIDs) == 0 {
		return []RetrieveResult{}, RetrieveMetrics{
			Query:      query,
			TopK:       topK,
			CorpusSize: 0,
		}, nil
	}

	searchQuery := normalizeSearchQuery(query)
	queryVector := vectordb.Embed(searchQuery, embedDims)
	results := make([]RetrieveResult, 0, len(chunkIDs))
	allCandidates := make([]RetrieveResult, 0, len(chunkIDs))

	// 从 Redis 获取向量数据
	for _, chunkID := range chunkIDs {
		chunkData, err := s.vectorStore.GetChunk(ctx, userID, chunkID)
		if err != nil {
			continue
		}

		content := chunkData["content"]
		vectorStr := chunkData["vector"]
		documentID := uint(0)
		if docIDStr, ok := chunkData["document_id"]; ok {
			fmt.Sscanf(docIDStr, "%d", &documentID)
		}

		// 反序列化向量
		vector, err := vectordb.UnmarshalVectorFromString(vectorStr)
		if err != nil {
			continue
		}

		score := cosine(queryVector, vector)
		candidate := RetrieveResult{
			ChunkID:    chunkID,
			DocumentID: documentID,
			Content:    content,
			Score:      score,
		}
		allCandidates = append(allCandidates, candidate)

		if score >= minScoreLimit {
			results = append(results, candidate)
		}
	}

	// 兜底策略：向量阈值未命中时，尝试关键词匹配，再放宽阈值。
	if len(results) == 0 {
		results = lexicalFallback(query, allCandidates, topK)
	}
	if len(results) == 0 {
		results = topByScore(allCandidates, topK, fallbackScoreLimit)
	}

	// 排序和限制结果数
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}

	metrics := RetrieveMetrics{
		Query:         query,
		TopK:          topK,
		CorpusSize:    len(chunkIDs),
		MatchedCount:  len(results),
		MinScoreLimit: minScoreLimit,
	}
	if len(results) > 0 {
		metrics.MaxScore = results[0].Score
		var sum float64
		for _, item := range results {
			sum += item.Score
		}
		metrics.AverageScore = sum / float64(len(results))
	}

	return results, metrics, nil
}

// retrieveFromMySQL 从 MySQL 检索相关文档
func (s *Service) retrieveFromMySQL(ctx context.Context, userID uint, query string, topK int) ([]RetrieveResult, RetrieveMetrics, error) {
	chunks, err := s.repo.ListChunks(ctx, userID, 1000)
	if err != nil {
		return nil, RetrieveMetrics{}, err
	}

	searchQuery := normalizeSearchQuery(query)
	queryVector := vectordb.Embed(searchQuery, embedDims)
	results := make([]RetrieveResult, 0, len(chunks))
	allCandidates := make([]RetrieveResult, 0, len(chunks))
	for _, chunk := range chunks {
		vector, unmarshalErr := vectordb.UnmarshalVector(chunk.Embedding)
		if unmarshalErr != nil {
			continue
		}
		score := cosine(queryVector, vector)
		candidate := RetrieveResult{ChunkID: chunk.ID, DocumentID: chunk.DocumentID, Content: chunk.Content, Score: score}
		allCandidates = append(allCandidates, candidate)
		if score < minScoreLimit {
			continue
		}
		results = append(results, candidate)
	}

	if len(results) == 0 {
		results = lexicalFallback(query, allCandidates, topK)
	}
	if len(results) == 0 {
		results = topByScore(allCandidates, topK, fallbackScoreLimit)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}

	metrics := RetrieveMetrics{
		Query:         query,
		TopK:          topK,
		CorpusSize:    len(chunks),
		MatchedCount:  len(results),
		MinScoreLimit: minScoreLimit,
	}
	if len(results) > 0 {
		metrics.MaxScore = results[0].Score
		var sum float64
		for _, item := range results {
			sum += item.Score
		}
		metrics.AverageScore = sum / float64(len(results))
	}

	return results, metrics, nil
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

func splitContent(content string, maxRunes, overlap int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	runes := []rune(content)
	if len(runes) <= maxRunes {
		return []string{content}
	}

	if overlap < 0 {
		overlap = 0
	}
	if overlap >= maxRunes {
		overlap = maxRunes / 4
	}

	step := maxRunes - overlap
	if step <= 0 {
		step = maxRunes
	}

	chunks := make([]string, 0)
	for start := 0; start < len(runes); start += step {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}

		// 优先在中文标点和句号处截断，减少语义断裂
		if end < len(runes) {
			for i := end; i > start+maxRunes/2; i-- {
				switch runes[i-1] {
				case '。', '！', '？', '.', '!', '?', '\n':
					end = i
					i = start
				}
			}
		}

		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			chunks = append(chunks, part)
		}
		if end >= len(runes) {
			break
		}
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

func normalizeSearchQuery(query string) string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return query
	}

	noise := []string{
		"请", "请问", "帮我", "帮忙", "查看", "看下", "看一下", "告诉我", "一下",
		"what", "is", "the", "for", "about", "please", "show", "me",
	}
	for _, n := range noise {
		query = strings.ReplaceAll(query, n, " ")
	}

	fields := strings.Fields(query)
	if len(fields) == 0 {
		return strings.TrimSpace(query)
	}
	return strings.Join(fields, " ")
}

func extractKeywords(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var b strings.Builder
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}

	stop := map[string]struct{}{
		"请": {}, "请问": {}, "帮我": {}, "帮忙": {}, "查看": {}, "看下": {}, "看一下": {}, "告诉我": {}, "一下": {},
		"what": {}, "is": {}, "the": {}, "for": {}, "about": {}, "show": {}, "me": {}, "please": {},
	}

	out := make([]string, 0)
	for _, token := range strings.Fields(b.String()) {
		if _, ok := stop[token]; ok {
			continue
		}
		if len([]rune(token)) < 2 {
			continue
		}
		out = append(out, token)
	}
	return out
}

func lexicalFallback(query string, candidates []RetrieveResult, topK int) []RetrieveResult {
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return nil
	}

	results := make([]RetrieveResult, 0)
	for _, c := range candidates {
		text := strings.ToLower(c.Content)
		hit := 0
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				hit++
			}
		}
		if hit == 0 {
			continue
		}
		c.Score = 0.2 + 0.05*float64(hit)
		results = append(results, c)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func topByScore(candidates []RetrieveResult, topK int, min float64) []RetrieveResult {
	results := make([]RetrieveResult, 0)
	for _, c := range candidates {
		if c.Score >= min {
			results = append(results, c)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func (s *Service) DeleteDocument(ctx context.Context, userID uint, documentID uint) error {
	// Get document to verify ownership
	doc, err := s.repo.GetDocumentByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return errors.New("document not found")
	}
	if doc.UserID != userID {
		return errors.New("unauthorized: document does not belong to user")
	}

	// Delete from Redis vector store if available (using Pipeline for optimization)
	if s.vectorStore != nil {
		if err := s.vectorStore.DeleteDocumentPipeline(ctx, userID, documentID); err != nil {
			// Log the error but continue with MySQL deletion
			// In production, you might want to handle this differently
			fmt.Printf("warning: failed to delete vectors from Redis: %v\n", err)
		}
	}

	// Delete from MySQL (chunks and document)
	if err := s.repo.DeleteDocument(ctx, userID, documentID); err != nil {
		return err
	}

	return nil
}

// GetPerformanceStats 获取性能统计信息
func (s *Service) GetPerformanceStats(ctx context.Context, userID uint) (PerformanceStats, error) {
	// 获取文档和分块统计
	docs, err := s.repo.ListDocuments(ctx, userID, 1000)
	if err != nil {
		return PerformanceStats{}, err
	}

	totalChunks := 0
	for _, doc := range docs {
		chunks, err := s.repo.GetChunksByDocumentID(ctx, doc.ID)
		if err != nil {
			continue
		}
		totalChunks += len(chunks)
	}

	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	avgIngestTime := float64(0)
	if s.totalIngestCalls > 0 {
		avgIngestTime = float64(s.totalIngestTimeMs) / float64(s.totalIngestCalls)
	}

	avgSearchTime := float64(0)
	if s.totalSearchCalls > 0 {
		avgSearchTime = float64(s.totalSearchTimeMs) / float64(s.totalSearchCalls)
	}

	return PerformanceStats{
		TotalDocuments:   len(docs),
		TotalChunks:      totalChunks,
		AvgIngestTimeMs:  avgIngestTime,
		AvgSearchTimeMs:  avgSearchTime,
		TotalIngestCalls: s.totalIngestCalls,
		TotalSearchCalls: s.totalSearchCalls,
		LastIngestTimeMs: s.lastIngestTimeMs,
		LastSearchTimeMs: s.lastSearchTimeMs,
	}, nil
}
