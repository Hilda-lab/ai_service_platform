package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort       string
	JWTSecret      string
	JWTExpireHours int
	MySQLDSN       string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RabbitMQURL    string
	AIProvider     string
	OpenAIAPIKey   string
	OpenAIBaseURL  string
	OpenAIModel    string
	OllamaBaseURL  string
	OllamaModel    string
	GeminiAPIKey   string
	GeminiBaseURL  string
	GeminiModel    string
	VisionProvider string
	VisionModel    string
	VisionMock     bool
	VisionQueue    string
}

func Load() Config {
	return Config{
		HTTPPort:       getEnv("HTTP_PORT", "8080"),
		JWTSecret:      getEnv("JWT_SECRET", "replace-with-strong-secret"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 72),
		MySQLDSN:       getEnv("MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/ai_platform?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:      getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		AIProvider:     getEnv("AI_PROVIDER", "openai"),
		OpenAIAPIKey:   getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:  getEnv("OPENAI_BASE_URL", "https://cdn.12ai.org"),
		OpenAIModel:    getEnv("OPENAI_MODEL", "gpt-5.1"),
		OllamaBaseURL:  getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		OllamaModel:    getEnv("OLLAMA_MODEL", "llama3.1"),
		GeminiAPIKey:   getEnv("GEMINI_API_KEY", ""),
		GeminiBaseURL:  getEnv("GEMINI_BASE_URL", "https://cdn.12ai.org"),
		GeminiModel:    getEnv("GEMINI_MODEL", "gemini-3-pro-preview"),
		VisionProvider: getEnv("VISION_PROVIDER", "openai"),
		VisionModel:    getEnv("VISION_MODEL", "gpt-4.1-mini"),
		VisionMock:     getEnvBool("VISION_MOCK", false),
		VisionQueue:    getEnv("VISION_QUEUE", "vision_tasks"),
	}
}

func (c Config) JWTDuration() time.Duration {
	return time.Duration(c.JWTExpireHours) * time.Hour
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
