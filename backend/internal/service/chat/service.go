package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"ai-service-platform/backend/internal/domain/entity"
	"ai-service-platform/backend/internal/domain/repository"
	ollamaclient "ai-service-platform/backend/internal/infrastructure/ai/ollama"
	openaiclient "ai-service-platform/backend/internal/infrastructure/ai/openai"
)

type Service struct {
	repo               repository.ChatRepository
	redis              *goredis.Client
	openai             *openaiclient.Client
	ollama             *ollamaclient.Client
	retriever          Retriever
	defaultProvider    string
	defaultOpenAIModel string
	defaultOllamaModel string
}

type Retriever interface {
	RetrieveContents(ctx context.Context, userID uint, query string, topK int) ([]string, error)
}

type ChatRequest struct {
	UserID    uint
	SessionID *uint
	Provider  string
	Model     string
	Message   string
	UseRAG    bool
}

type ChatResult struct {
	SessionID uint
	Provider  string
	Model     string
	Reply     string
}

type contextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewService(repo repository.ChatRepository, redis *goredis.Client, openai *openaiclient.Client, ollama *ollamaclient.Client, retriever Retriever, defaultProvider, defaultOpenAIModel, defaultOllamaModel string) *Service {
	if defaultProvider == "" {
		defaultProvider = "openai"
	}
	if defaultOpenAIModel == "" {
		defaultOpenAIModel = "gpt-5.1"
	}
	if defaultOllamaModel == "" {
		defaultOllamaModel = "llama3.1"
	}
	return &Service{
		repo:               repo,
		redis:              redis,
		openai:             openai,
		ollama:             ollama,
		retriever:          retriever,
		defaultProvider:    defaultProvider,
		defaultOpenAIModel: defaultOpenAIModel,
		defaultOllamaModel: defaultOllamaModel,
	}
}

func (s *Service) ListSessions(ctx context.Context, userID uint) ([]entity.ChatSession, error) {
	return s.repo.ListSessions(ctx, userID, 100)
}

func (s *Service) ListMessages(ctx context.Context, userID, sessionID uint) ([]entity.ChatMessage, error) {
	session, err := s.repo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("session not found")
	}
	return s.repo.ListMessages(ctx, sessionID, userID, 200)
}

func (s *Service) Complete(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	session, provider, model, err := s.ensureSession(ctx, req)
	if err != nil {
		return nil, err
	}

	userContent := strings.TrimSpace(req.Message)
	if userContent == "" {
		return nil, errors.New("message is required")
	}

	if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
		SessionID: session.ID,
		UserID:    req.UserID,
		Role:      "user",
		Content:   userContent,
		Provider:  provider,
		Model:     model,
	}); err != nil {
		return nil, err
	}

	history, err := s.loadContext(ctx, req.UserID, session.ID)
	if err != nil {
		return nil, err
	}
	history, err = s.enhanceWithRAG(ctx, req.UserID, req.Message, req.UseRAG, history)
	if err != nil {
		return nil, err
	}
	history = append(history, contextMessage{Role: "user", Content: userContent})

	reply, err := s.generateReply(ctx, provider, model, history)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
		SessionID: session.ID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   reply,
		Provider:  provider,
		Model:     model,
	}); err != nil {
		return nil, err
	}

	if err := s.refreshCache(ctx, req.UserID, session.ID); err != nil {
		return nil, err
	}

	return &ChatResult{SessionID: session.ID, Provider: provider, Model: model, Reply: reply}, nil
}

func (s *Service) Stream(ctx context.Context, req ChatRequest, onChunk func(chunk string) error) (*ChatResult, error) {
	session, provider, model, err := s.ensureSession(ctx, req)
	if err != nil {
		return nil, err
	}

	userContent := strings.TrimSpace(req.Message)
	if userContent == "" {
		return nil, errors.New("message is required")
	}

	if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
		SessionID: session.ID,
		UserID:    req.UserID,
		Role:      "user",
		Content:   userContent,
		Provider:  provider,
		Model:     model,
	}); err != nil {
		return nil, err
	}

	history, err := s.loadContext(ctx, req.UserID, session.ID)
	if err != nil {
		return nil, err
	}
	history, err = s.enhanceWithRAG(ctx, req.UserID, req.Message, req.UseRAG, history)
	if err != nil {
		return nil, err
	}
	history = append(history, contextMessage{Role: "user", Content: userContent})

	var builder strings.Builder
	if provider == "openai" {
		err = s.streamOpenAI(ctx, model, history, func(chunk string) error {
			if chunk == "" {
				return nil
			}
			builder.WriteString(chunk)
			return onChunk(chunk)
		})
	} else if provider == "ollama" {
		err = s.streamOllama(ctx, model, history, func(chunk string) error {
			if chunk == "" {
				return nil
			}
			builder.WriteString(chunk)
			return onChunk(chunk)
		})
	} else {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
	if err != nil {
		return nil, err
	}

	reply := builder.String()
	if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
		SessionID: session.ID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   reply,
		Provider:  provider,
		Model:     model,
	}); err != nil {
		return nil, err
	}

	if err := s.refreshCache(ctx, req.UserID, session.ID); err != nil {
		return nil, err
	}

	return &ChatResult{SessionID: session.ID, Provider: provider, Model: model, Reply: reply}, nil
}

func (s *Service) ensureSession(ctx context.Context, req ChatRequest) (*entity.ChatSession, string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = s.defaultProvider
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		if provider == "ollama" {
			model = s.defaultOllamaModel
		} else {
			model = s.defaultOpenAIModel
		}
	}

	if req.SessionID != nil {
		session, err := s.repo.GetSessionByID(ctx, *req.SessionID, req.UserID)
		if err != nil {
			return nil, "", "", err
		}
		if session == nil {
			return nil, "", "", errors.New("session not found")
		}
		return session, provider, model, nil
	}

	title := strings.TrimSpace(req.Message)
	if title == "" {
		title = "新会话"
	}
	if len([]rune(title)) > 24 {
		title = string([]rune(title)[:24]) + "..."
	}

	session := &entity.ChatSession{
		UserID:   req.UserID,
		Title:    title,
		Provider: provider,
		Model:    model,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, "", "", err
	}
	return session, provider, model, nil
}

func (s *Service) loadContext(ctx context.Context, userID, sessionID uint) ([]contextMessage, error) {
	if s.redis != nil {
		if data, err := s.redis.Get(ctx, s.contextKey(sessionID)).Result(); err == nil && data != "" {
			var cached []contextMessage
			if jsonErr := json.Unmarshal([]byte(data), &cached); jsonErr == nil {
				return cached, nil
			}
		}
	}

	messages, err := s.repo.ListMessages(ctx, sessionID, userID, 20)
	if err != nil {
		return nil, err
	}

	ctxMessages := make([]contextMessage, 0, len(messages))
	for _, message := range messages {
		ctxMessages = append(ctxMessages, contextMessage{Role: message.Role, Content: message.Content})
	}
	return ctxMessages, nil
}

func (s *Service) refreshCache(ctx context.Context, userID, sessionID uint) error {
	if s.redis == nil {
		return nil
	}

	messages, err := s.repo.ListMessages(ctx, sessionID, userID, 20)
	if err != nil {
		return err
	}

	ctxMessages := make([]contextMessage, 0, len(messages))
	for _, message := range messages {
		ctxMessages = append(ctxMessages, contextMessage{Role: message.Role, Content: message.Content})
	}

	payload, err := json.Marshal(ctxMessages)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, s.contextKey(sessionID), string(payload), 0).Err()
}

func (s *Service) generateReply(ctx context.Context, provider, model string, messages []contextMessage) (string, error) {
	if provider == "openai" {
		reqMessages := make([]openaiclient.ChatMessage, 0, len(messages))
		for _, message := range messages {
			reqMessages = append(reqMessages, openaiclient.ChatMessage{Role: message.Role, Content: message.Content})
		}
		resp, err := s.openai.ChatCompletions(ctx, openaiclient.ChatCompletionRequest{Model: model, Messages: reqMessages})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", errors.New("empty response")
		}
		return resp.Choices[0].Message.Content, nil
	}

	if provider == "ollama" {
		reqMessages := make([]ollamaclient.ChatMessage, 0, len(messages))
		for _, message := range messages {
			reqMessages = append(reqMessages, ollamaclient.ChatMessage{Role: message.Role, Content: message.Content})
		}
		resp, err := s.ollama.Chat(ctx, ollamaclient.ChatRequest{Model: model, Messages: reqMessages})
		if err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	}

	return "", fmt.Errorf("unsupported provider: %s", provider)
}

func (s *Service) streamOpenAI(ctx context.Context, model string, messages []contextMessage, onChunk func(chunk string) error) error {
	reqMessages := make([]openaiclient.ChatMessage, 0, len(messages))
	for _, message := range messages {
		reqMessages = append(reqMessages, openaiclient.ChatMessage{Role: message.Role, Content: message.Content})
	}

	return s.openai.ChatCompletionsStream(ctx, openaiclient.ChatCompletionRequest{Model: model, Messages: reqMessages}, func(line string) error {
		if !strings.HasPrefix(line, "data:") {
			return nil
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil
		}
		if len(chunk.Choices) == 0 {
			return nil
		}
		return onChunk(chunk.Choices[0].Delta.Content)
	})
}

func (s *Service) streamOllama(ctx context.Context, model string, messages []contextMessage, onChunk func(chunk string) error) error {
	reqMessages := make([]ollamaclient.ChatMessage, 0, len(messages))
	for _, message := range messages {
		reqMessages = append(reqMessages, ollamaclient.ChatMessage{Role: message.Role, Content: message.Content})
	}

	return s.ollama.ChatStream(ctx, ollamaclient.ChatRequest{Model: model, Messages: reqMessages}, func(content string, _ bool) error {
		return onChunk(content)
	})
}

func (s *Service) contextKey(sessionID uint) string {
	return fmt.Sprintf("chat:session:%d:context", sessionID)
}

func (s *Service) enhanceWithRAG(ctx context.Context, userID uint, query string, useRAG bool, history []contextMessage) ([]contextMessage, error) {
	if !useRAG || s.retriever == nil {
		return history, nil
	}

	contents, err := s.retriever.RetrieveContents(ctx, userID, query, 3)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return history, nil
	}

	var builder strings.Builder
	builder.WriteString("请优先基于以下知识库片段回答，若知识不足请明确说明。\n")
	for index, content := range contents {
		builder.WriteString(fmt.Sprintf("[%d] %s\n", index+1, content))
	}

	enhanced := make([]contextMessage, 0, len(history)+1)
	enhanced = append(enhanced, contextMessage{Role: "system", Content: builder.String()})
	enhanced = append(enhanced, history...)
	return enhanced, nil
}
