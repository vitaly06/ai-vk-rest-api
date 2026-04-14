package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	VK      VKConfig
	DB      DBConfig
	Redis   RedisConfig
	AI      AIConfig
	Payment PaymentConfig
	Server  ServerConfig
	Bot     BotConfig
}

type VKConfig struct {
	Token             string
	GroupID           int
	ConfirmationToken string
	Secret            string
	AdminIDs          []int
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AIConfig struct {
	Provider     string
	BaseURL      string
	APIKey       string
	ProjectID    string
	Model        string
	MaxTokens    int
	SystemPrompt string
}

type PaymentConfig struct {
	YooKassaShopID    string
	YooKassaSecretKey string
	ReturnURL         string
}

type ServerConfig struct {
	Host string
	Port string
}

type BotConfig struct {
	Mode             string // longpoll | callback
	InviteLinkTTLHrs int
}

func Load() *Config {
	aiProvider := strings.ToLower(getEnv("AI_PROVIDER", "openai"))
	aiBaseURL := os.Getenv("AI_BASE_URL")
	if aiBaseURL == "" {
		if aiProvider == "yandex" {
			aiBaseURL = "https://ai.api.cloud.yandex.net/v1"
		} else {
			aiBaseURL = "https://api.openai.com/v1"
		}
	}
	aiAPIKey := firstNonEmpty(os.Getenv("AI_API_KEY"), os.Getenv("YANDEX_API_KEY"))
	aiProjectID := firstNonEmpty(os.Getenv("AI_PROJECT_ID"), os.Getenv("YANDEX_FOLDER_ID"))
	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" {
		if aiProvider == "yandex" {
			aiModel = "yandexgpt-lite/latest"
		} else {
			aiModel = "gpt-4o"
		}
	}

	return &Config{
		VK: VKConfig{
			Token:             mustEnv("VK_TOKEN"),
			GroupID:           mustEnvInt("VK_GROUP_ID"),
			ConfirmationToken: os.Getenv("VK_CONFIRMATION_TOKEN"),
			Secret:            os.Getenv("VK_SECRET"),
			AdminIDs:          parseIntList(os.Getenv("ADMIN_IDS")),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "ai_vk_bot"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		AI: AIConfig{
			Provider:     aiProvider,
			BaseURL:      aiBaseURL,
			APIKey:       aiAPIKey,
			ProjectID:    aiProjectID,
			Model:        aiModel,
			MaxTokens:    getEnvInt("AI_MAX_TOKENS", 2048),
			SystemPrompt: loadSystemPrompt(),
		},
		Payment: PaymentConfig{
			YooKassaShopID:    os.Getenv("YOOKASSA_SHOP_ID"),
			YooKassaSecretKey: os.Getenv("YOOKASSA_SECRET_KEY"),
			ReturnURL:         os.Getenv("PAYMENT_RETURN_URL"),
		},
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Bot: BotConfig{
			Mode:             getEnv("BOT_MODE", "longpoll"),
			InviteLinkTTLHrs: getEnvInt("INVITE_LINK_TTL_HOURS", 72),
		},
	}
}

func (d DBConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + d.Port +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.Name +
		" sslmode=" + d.SSLMode
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env variable not set: " + key)
	}
	return v
}

func mustEnvInt(key string) int {
	v := mustEnv(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		panic("env variable " + key + " must be an integer")
	}
	return n
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseIntList(s string) []int {
	var result []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err == nil {
			result = append(result, n)
		}
	}
	return result
}

// loadSystemPrompt читает промпт из файла (AI_SYSTEM_PROMPT_FILE) или из переменной (AI_SYSTEM_PROMPT).
func loadSystemPrompt() string {
	if path := os.Getenv("AI_SYSTEM_PROMPT_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("warning: cannot read AI_SYSTEM_PROMPT_FILE %q: %v", path, err)
		} else {
			return strings.TrimSpace(string(data))
		}
	}
	return getEnv("AI_SYSTEM_PROMPT", "You are a helpful assistant.")
}
