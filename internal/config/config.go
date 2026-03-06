package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort  string
	DatabaseURL string

	MetaVerifyToken string
	MetaAppSecret   string

	LLMProvider string
	LLMApiKey   string
	LLMModel    string

	LogLevel string
}

func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if missing)
	_ = godotenv.Load()

	cfg := &Config{
		ServerPort:  getEnv("SERVER_PORT", "3000"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		MetaVerifyToken: getEnv("META_VERIFY_TOKEN", ""),
		MetaAppSecret:   getEnv("META_APP_SECRET", ""),

		LLMProvider: getEnv("LLM_PROVIDER", "openai"),
		LLMApiKey:   getEnv("LLM_API_KEY", ""),
		LLMModel:    getEnv("LLM_MODEL", "gpt-4o"),

		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
