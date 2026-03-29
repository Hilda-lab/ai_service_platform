package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	chatservice "ai-service-platform/backend/internal/service/chat"
	ragservice "ai-service-platform/backend/internal/service/rag"
)

var weatherHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Tool 定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Execute     func(context.Context, json.RawMessage) (interface{}, error) `json:"-"`
}

type Hub struct {
	chat  *chatservice.Service
	rag   *ragservice.Service
	tools map[string]*Tool
}

type Client struct {
	userID uint
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
}

type rpcRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID     string      `json:"id,omitempty"`
	Type   string      `json:"type,omitempty"`
	Method string      `json:"method,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type chatSendParams struct {
	SessionID *uint  `json:"session_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Message   string `json:"message"`
	UseRAG    bool   `json:"use_rag"`
}

type chatMessagesParams struct {
	SessionID uint `json:"session_id"`
}

type toolCallParams struct {
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
}

func NewHub(chat *chatservice.Service, rag *ragservice.Service) *Hub {
	h := &Hub{
		chat:  chat,
		rag:   rag,
		tools: make(map[string]*Tool),
	}
	h.registerTools()
	return h
}

// 注册所有可用工具
func (h *Hub) registerTools() {
	// 工具 1: 获取当前时间
	h.tools["get_datetime"] = &Tool{
		Name:        "get_datetime",
		Description: "获取当前日期和时间，返回 RFC3339 格式的时间戳",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "时区，例如 UTC、Asia/Shanghai，默认为 UTC",
				},
			},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var p struct {
				Timezone string `json:"timezone"`
			}
			if len(params) > 0 {
				if err := json.Unmarshal(params, &p); err != nil {
					return nil, fmt.Errorf("invalid params: %v", err)
				}
			}

			loc := time.UTC
			tz := "UTC"
			if strings.TrimSpace(p.Timezone) != "" {
				loaded, err := time.LoadLocation(strings.TrimSpace(p.Timezone))
				if err != nil {
					return nil, fmt.Errorf("invalid timezone: %s", p.Timezone)
				}
				loc = loaded
				tz = loaded.String()
			}

			now := time.Now().In(loc)
			return map[string]interface{}{
				"current_time":   now.Format(time.RFC3339),
				"unix_timestamp": now.Unix(),
				"timezone":       tz,
			}, nil
		},
	}

	// 工具 2: 查询 RAG 知识库
	h.tools["query_rag"] = &Tool{
		Name:        "query_rag",
		Description: "查询 RAG 知识库中的文档，基于相似性检索返回相关内容",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "查询关键词或问题",
				},
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "返回最相似的前 K 个结果，默认为 3",
				},
			},
			"required": []string{"query"},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			// 注意：这个工具在实际调用时会在 handleRequest 中进行特殊处理
			// 这里只返回静态响应，真实调用需要 userID
			return map[string]interface{}{
				"message": "此工具需要在 handleRequest 中使用 client.userID 进行调用",
				"results": []interface{}{},
			}, nil
		},
	}

	// 工具 3: 获取系统信息
	h.tools["query_system_info"] = &Tool{
		Name:        "query_system_info",
		Description: "获取系统相关信息，如操作系统、Go 版本等",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"fields": map[string]interface{}{
					"type":        "array",
					"description": "要查询的字段：os, arch, go_version, hostname。不指定则返回所有",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var p struct {
				Fields []string `json:"fields"`
			}
			json.Unmarshal(params, &p)

			info := map[string]interface{}{
				"os":        runtime.GOOS,
				"arch":      runtime.GOARCH,
				"go_version": runtime.Version(),
			}

			if hostname, err := os.Hostname(); err == nil {
				info["hostname"] = hostname
			}

			info["current_time"] = time.Now().Format(time.RFC3339)
			return info, nil
		},
	}

	// 工具 4: 查询天气
	h.tools["query_weather"] = &Tool{
		Name:        "query_weather",
		Description: "查询指定城市或经纬度的实时天气信息",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"city": map[string]interface{}{
					"type":        "string",
					"description": "城市名称，例如 Shanghai、Beijing",
				},
				"latitude": map[string]interface{}{
					"type":        "number",
					"description": "纬度，可选。与 longitude 一起传入时优先使用经纬度",
				},
				"longitude": map[string]interface{}{
					"type":        "number",
					"description": "经度，可选。与 latitude 一起传入时优先使用经纬度",
				},
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "时区，例如 Asia/Shanghai，默认 auto",
				},
			},
		},
		Execute: func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			return fetchWeather(ctx, params)
		},
	}
}

func fetchWeather(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		City      string   `json:"city"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		Timezone  string   `json:"timezone"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %v", err)
		}
	}

	timezone := strings.TrimSpace(p.Timezone)
	if timezone == "" {
		timezone = "auto"
	}

	lat, lon, locationName, err := resolveLocation(ctx, p.City, p.Latitude, p.Longitude)
	if err != nil {
		return nil, err
	}

	weatherURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true&timezone=%s", lat, lon, url.QueryEscape(timezone))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, weatherURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather service unavailable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("weather api returned status %d", resp.StatusCode)
	}

	var weatherResp struct {
		Timezone       string `json:"timezone"`
		CurrentWeather struct {
			Temperature float64 `json:"temperature"`
			Windspeed   float64 `json:"windspeed"`
			Winddir     float64 `json:"winddirection"`
			WeatherCode int     `json:"weathercode"`
			Time        string  `json:"time"`
		} `json:"current_weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return nil, fmt.Errorf("decode weather response failed: %v", err)
	}

	return map[string]interface{}{
		"location": map[string]interface{}{
			"name":      locationName,
			"latitude":  lat,
			"longitude": lon,
		},
		"timezone":          weatherResp.Timezone,
		"observation_time":  weatherResp.CurrentWeather.Time,
		"temperature_c":     weatherResp.CurrentWeather.Temperature,
		"wind_speed_kmh":    weatherResp.CurrentWeather.Windspeed,
		"wind_direction_deg": weatherResp.CurrentWeather.Winddir,
		"weather_code":      weatherResp.CurrentWeather.WeatherCode,
	}, nil
}

func resolveLocation(ctx context.Context, city string, latitude, longitude *float64) (float64, float64, string, error) {
	if latitude != nil && longitude != nil {
		return *latitude, *longitude, "custom", nil
	}

	name := strings.TrimSpace(city)
	if name == "" {
		name = "Shanghai"
	}

	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json", url.QueryEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return 0, 0, "", err
	}
	resp, err := weatherHTTPClient.Do(req)
	if err != nil {
		return 0, 0, "", fmt.Errorf("geocoding service unavailable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, "", fmt.Errorf("geocoding api returned status %d", resp.StatusCode)
	}

	var geoResp struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		return 0, 0, "", fmt.Errorf("decode geocoding response failed: %v", err)
	}
	if len(geoResp.Results) == 0 {
		return 0, 0, "", fmt.Errorf("city not found: %s", name)
	}

	best := geoResp.Results[0]
	fullName := best.Name
	if strings.TrimSpace(best.Country) != "" {
		fullName = fmt.Sprintf("%s, %s", best.Name, best.Country)
	}
	return best.Latitude, best.Longitude, fullName, nil
}

func (h *Hub) ServeConnection(ctx context.Context, userID uint, conn *websocket.Conn) {
	client := &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 32),
		hub:    h,
	}

	go client.writePump()
	client.sendResponse(rpcResponse{Type: "welcome", Result: map[string]interface{}{"user_id": userID, "ts": time.Now().Unix()}})
	client.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer close(c.send)
	defer c.conn.Close()

	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var req rpcRequest
		if err := json.Unmarshal(message, &req); err != nil {
			c.sendResponse(rpcResponse{Type: "error", Error: "invalid json request"})
			continue
		}

		c.handleRequest(ctx, req)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, req rpcRequest) {
	switch req.Method {
	case "ping":
		c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{"pong": true, "ts": time.Now().Unix()}})
	
	// 工具相关方法
	case "tool.list":
		// 返回可用工具列表
		toolList := make([]map[string]interface{}, 0)
		for _, tool := range c.hub.tools {
			toolList = append(toolList, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			})
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{
			"tools": toolList,
			"count": len(toolList),
		}})
	
	case "tool.call":
		// 执行指定工具
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid params"})
			return
		}

		// 特殊处理 query_rag，需要 userID
		if params.ToolName == "query_rag" {
			var ragParams struct {
				Query string `json:"query"`
				TopK  int    `json:"top_k"`
			}
			if err := json.Unmarshal(params.Args, &ragParams); err != nil {
				c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid query_rag params"})
				return
			}
			if ragParams.TopK == 0 {
				ragParams.TopK = 3
			}

			if c.hub.rag == nil {
				c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{
					"results": []interface{}{},
					"message": "RAG service not configured",
				}})
				return
			}

			results, err := c.hub.rag.Retrieve(ctx, c.userID, ragParams.Query, ragParams.TopK)
			if err != nil {
				c.sendResponse(rpcResponse{ID: req.ID, Error: fmt.Sprintf("rag retrieve error: %v", err)})
				return
			}

			c.sendResponse(rpcResponse{ID: req.ID, Result: map[string]interface{}{
				"query":   ragParams.Query,
				"results": results,
				"count":   len(results),
			}})
			return
		}

		// 其他工具使用通用执行方式
		tool, exists := c.hub.tools[params.ToolName]
		if !exists {
			c.sendResponse(rpcResponse{ID: req.ID, Error: fmt.Sprintf("tool not found: %s", params.ToolName)})
			return
		}

		result, err := tool.Execute(ctx, params.Args)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}

		c.sendResponse(rpcResponse{ID: req.ID, Result: result})
	
	// 聊天相关方法
	case "chat.send":
		var params chatSendParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid params"})
			return
		}

		result, err := c.hub.chat.Complete(ctx, chatservice.ChatRequest{
			UserID:    c.userID,
			SessionID: params.SessionID,
			Provider:  params.Provider,
			Model:     params.Model,
			Message:   params.Message,
			UseRAG:    params.UseRAG,
		})
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}

		c.sendResponse(rpcResponse{ID: req.ID, Result: result})
		c.sendResponse(rpcResponse{Type: "event", Method: "chat.message", Result: result})
	case "chat.sessions":
		sessions, err := c.hub.chat.ListSessions(ctx, c.userID)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: sessions})
	case "chat.messages":
		var params chatMessagesParams
		if err := json.Unmarshal(req.Params, &params); err != nil || params.SessionID == 0 {
			c.sendResponse(rpcResponse{ID: req.ID, Error: "invalid session_id"})
			return
		}
		messages, err := c.hub.chat.ListMessages(ctx, c.userID, params.SessionID)
		if err != nil {
			c.sendResponse(rpcResponse{ID: req.ID, Error: err.Error()})
			return
		}
		c.sendResponse(rpcResponse{ID: req.ID, Result: messages})
	default:
		c.sendResponse(rpcResponse{ID: req.ID, Error: "unsupported method"})
	}
}

func (c *Client) sendResponse(resp rpcResponse) {
	payload, err := json.Marshal(resp)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
	}
}

// ==================== 实现 ToolProvider 接口 ====================

// GetTools 返回所有可用工具，供聊天服务的 Function Calling 使用
func (h *Hub) GetTools(ctx context.Context, userID uint) ([]map[string]interface{}, error) {
	// 返回工具列表
	tools := make([]map[string]interface{}, 0, len(h.tools))
	for _, tool := range h.tools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}
	return tools, nil
}

// ExecuteTool 执行指定的工具
func (h *Hub) ExecuteTool(ctx context.Context, userID uint, toolName string, arguments json.RawMessage) (interface{}, error) {
	tool, exists := h.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// 特殊处理 query_rag，需要 userID
	if toolName == "query_rag" {
		var ragParams struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
		}
		if err := json.Unmarshal(arguments, &ragParams); err != nil {
			return nil, fmt.Errorf("invalid query_rag params: %v", err)
		}
		if ragParams.TopK == 0 {
			ragParams.TopK = 3
		}

		if h.rag == nil {
			return nil, errors.New("RAG service not configured")
		}

		results, err := h.rag.Retrieve(ctx, userID, ragParams.Query, ragParams.TopK)
		if err != nil {
			return nil, fmt.Errorf("rag retrieve error: %v", err)
		}

		return map[string]interface{}{
			"query":   ragParams.Query,
			"results": results,
			"count":   len(results),
		}, nil
	}

	// 其他工具使用通用执行方式
	result, err := tool.Execute(ctx, arguments)
	if err != nil {
		return nil, fmt.Errorf("tool execution error: %v", err)
	}

	return result, nil
}
