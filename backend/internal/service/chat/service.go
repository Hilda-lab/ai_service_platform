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
	toolProvider       ToolProvider
}

type Retriever interface {
	RetrieveContents(ctx context.Context, userID uint, query string, topK int) ([]string, error)
}

// ToolProvider 接口用于从 MCP Hub 获取可用工具
type ToolProvider interface {
	GetTools(ctx context.Context, userID uint) ([]map[string]interface{}, error)
	ExecuteTool(ctx context.Context, userID uint, toolName string, arguments json.RawMessage) (interface{}, error)
}

type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
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
	Usage     *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
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
		toolProvider:       nil,
	}
}

// SetToolProvider 设置工具提供者（用于 Function Calling）
func (s *Service) SetToolProvider(tp ToolProvider) {
	s.toolProvider = tp
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

	if directReply, handled, err := s.tryDirectToolInvocation(ctx, req.UserID, userContent); err != nil {
		return nil, err
	} else if handled {
		if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
			SessionID: session.ID,
			UserID:    req.UserID,
			Role:      "assistant",
			Content:   directReply,
			Provider:  provider,
			Model:     model,
		}); err != nil {
			return nil, err
		}

		if err := s.refreshCache(ctx, req.UserID, session.ID); err != nil {
			return nil, err
		}

		return &ChatResult{SessionID: session.ID, Provider: provider, Model: model, Reply: directReply}, nil
	}

	reply, usage, err := s.generateReply(ctx, req.UserID, provider, model, history)
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

	result := &ChatResult{SessionID: session.ID, Provider: provider, Model: model, Reply: reply}
	if usage != nil {
		result.Usage = &struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	}
	return result, nil
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

	if directReply, handled, err := s.tryDirectToolInvocation(ctx, req.UserID, userContent); err != nil {
		return nil, err
	} else if handled {
		if err := onChunk(directReply); err != nil {
			return nil, err
		}
		if err := s.repo.CreateMessage(ctx, &entity.ChatMessage{
			SessionID: session.ID,
			UserID:    req.UserID,
			Role:      "assistant",
			Content:   directReply,
			Provider:  provider,
			Model:     model,
		}); err != nil {
			return nil, err
		}

		if err := s.refreshCache(ctx, req.UserID, session.ID); err != nil {
			return nil, err
		}

		return &ChatResult{SessionID: session.ID, Provider: provider, Model: model, Reply: directReply}, nil
	}

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

func (s *Service) generateReply(ctx context.Context, userID uint, provider, model string, messages []contextMessage) (string, *struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}, error) {
	if provider == "openai" {
		return s.generateReplyWithTools(ctx, userID, model, messages)
	}

	if provider == "ollama" {
		reqMessages := make([]ollamaclient.ChatMessage, 0, len(messages))
		for _, message := range messages {
			reqMessages = append(reqMessages, ollamaclient.ChatMessage{Role: message.Role, Content: message.Content})
		}
		resp, err := s.ollama.Chat(ctx, ollamaclient.ChatRequest{Model: model, Messages: reqMessages})
		if err != nil {
			return "", nil, err
		}
		return resp.Message.Content, nil, nil
	}

	return "", nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (s *Service) tryDirectToolInvocation(ctx context.Context, userID uint, userContent string) (string, bool, error) {
	if s.toolProvider == nil {
		return "", false, nil
	}

	normalized := strings.ToLower(strings.TrimSpace(userContent))

	toolName := ""
	args := map[string]interface{}{}

	manualCallMode := strings.Contains(userContent, "调用") || strings.Contains(normalized, "call ")

	if !manualCallMode {
		switch detectNaturalToolIntent(normalized, userContent) {
		case "get_datetime":
			toolName = "get_datetime"
			args["timezone"] = "Asia/Shanghai"
		case "query_weather":
			toolName = "query_weather"
			args["timezone"] = "Asia/Shanghai"
			args["city"] = detectCity(userContent)
		case "query_system_info":
			toolName = "query_system_info"
		default:
			return "", false, nil
		}
	}

	if manualCallMode {
		switch {
		case strings.Contains(normalized, "get_datetime"):
			toolName = "get_datetime"
			if strings.Contains(normalized, "asia/shanghai") || strings.Contains(userContent, "北京时间") {
				args["timezone"] = "Asia/Shanghai"
			}
			if _, ok := args["timezone"]; !ok {
				args["timezone"] = "Asia/Shanghai"
			}
		case strings.Contains(normalized, "query_weather"):
			toolName = "query_weather"
			args["timezone"] = "Asia/Shanghai"
			args["city"] = detectCity(userContent)
		case strings.Contains(normalized, "query_system_info"):
			toolName = "query_system_info"
		case strings.Contains(normalized, "query_rag"):
			toolName = "query_rag"
			args["query"] = userContent
			args["top_k"] = 3
		default:
			return "", false, nil
		}
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", true, err
	}

	result, err := s.toolProvider.ExecuteTool(ctx, userID, toolName, argsJSON)
	if err != nil {
		return "", true, err
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", true, err
	}

	reply := fmt.Sprintf("已调用工具 %s，返回结果：\n%s", toolName, string(payload))
	return reply, true, nil
}

func detectCity(text string) string {
	switch {
	case strings.Contains(text, "北京"):
		return "Beijing"
	case strings.Contains(text, "广州"):
		return "Guangzhou"
	case strings.Contains(text, "深圳"):
		return "Shenzhen"
	case strings.Contains(text, "杭州"):
		return "Hangzhou"
	case strings.Contains(text, "成都"):
		return "Chengdu"
	default:
		return "Shanghai"
	}
}

func detectNaturalToolIntent(normalized, original string) string {
	timeHints := []string{"现在几点", "现在几", "当前时间", "北京时间", "几点了", "what time", "current time"}
	for _, hint := range timeHints {
		if strings.Contains(normalized, strings.ToLower(hint)) || strings.Contains(original, hint) {
			return "get_datetime"
		}
	}

	weatherHints := []string{"天气", "气温", "温度", "会下雨", "weather", "forecast"}
	for _, hint := range weatherHints {
		if strings.Contains(normalized, strings.ToLower(hint)) || strings.Contains(original, hint) {
			return "query_weather"
		}
	}

	systemHints := []string{"系统信息", "go版本", "go version", "操作系统", "hostname"}
	for _, hint := range systemHints {
		if strings.Contains(normalized, strings.ToLower(hint)) || strings.Contains(original, hint) {
			return "query_system_info"
		}
	}

	return ""
}

func (s *Service) generateReplyWithTools(ctx context.Context, userID uint, model string, messages []contextMessage) (string, *struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}, error) {
	// 转换消息格式
	reqMessages := make([]openaiclient.ChatMessage, 0, len(messages))
	for _, message := range messages {
		reqMessages = append(reqMessages, openaiclient.ChatMessage{Role: message.Role, Content: message.Content})
	}

	// 获取可用工具（如果配置了 toolProvider）
	var tools []openaiclient.ToolDefinition
	if s.toolProvider != nil {
		toolInfos, err := s.toolProvider.GetTools(ctx, userID)
		if err == nil && len(toolInfos) > 0 {
			tools = make([]openaiclient.ToolDefinition, 0, len(toolInfos))
			for _, info := range toolInfos {
				var toolDef openaiclient.ToolDefinition
				toolDef.Type = "function"
				toolDef.Function.Name, _ = info["name"].(string)
				toolDef.Function.Description, _ = info["description"].(string)
				if params, ok := info["parameters"].(map[string]interface{}); ok {
					toolDef.Function.Parameters = params
				}
				tools = append(tools, toolDef)
			}
		}
	}

	// 最多5次迭代以防止无限循环
	var lastUsage *struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}

	for i := 0; i < 5; i++ {
		req := openaiclient.ChatCompletionRequest{
			Model:    model,
			Messages: reqMessages,
			Tools:    tools,
		}

		// 调试输出：记录工具信息
		if len(tools) > 0 {
			fmt.Printf("[DEBUG] Iteration %d: Sending %d tools to AI\n", i, len(tools))
			for _, t := range tools {
				fmt.Printf("  - Tool: %s\n", t.Function.Name)
			}
		}

		resp, err := s.openai.ChatCompletions(ctx, req)
		if err != nil {
			fmt.Printf("[ERROR] Iteration %d: OpenAI ChatCompletions failed: %v\n", i, err)
			return "", nil, err
		}

		if len(resp.Choices) == 0 {
			return "", nil, errors.New("empty response")
		}

		choice := resp.Choices[0]

		// 保存 usage 信息（最后一次重要）
		if resp.Usage != nil {
			lastUsage = &struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
			}{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			}
		}

		// 调试输出：记录 AI 的响应
		fmt.Printf("[DEBUG] Iteration %d: FinishReason=%s, ToolCalls=%d, Usage=%+v\n", i, choice.FinishReason, len(choice.Message.ToolCalls), resp.Usage)

		// 检查是否需要调用工具
		if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
			// 添加助手响应到消息历史
			reqMessages = append(reqMessages, openaiclient.ChatMessage{
				Role:      "assistant",
				ToolCalls: choice.Message.ToolCalls,
			})

			// 执行所有工具调用
			for _, toolCall := range choice.Message.ToolCalls {
				// 解析工具参数
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					args = map[string]interface{}{}
				}
				argsJSON, _ := json.Marshal(args)

				// 执行工具
				result, err := s.toolProvider.ExecuteTool(ctx, userID, toolCall.Function.Name, argsJSON)
				if err != nil {
					result = map[string]interface{}{"error": err.Error()}
				}

				// 将工具结果添加到消息历史
				resultJSON, _ := json.Marshal(result)
				reqMessages = append(reqMessages, openaiclient.ChatMessage{
					Role:       "user",
					ToolCallID: toolCall.ID,
					Name:       toolCall.Function.Name,
					Content:    string(resultJSON),
				})
			}

			// 继续循环，让 AI 生成最终响应
			continue
		}

		// 获得文本响应
		return choice.Message.Content, lastUsage, nil
	}

	return "", lastUsage, errors.New("too many tool calls, aborting")
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
