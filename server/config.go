package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application settings from environment variables.
type Config struct {
	PythonServiceURL string // POST multipart -> image classification
	PythonRAGURL     string // POST JSON {"question"} -> retrieval context in Python
	LLMAPIKey        string
	LLMModel         string
	LLMBaseURL       string
	ServerPort       string

	TelegramBotToken       string
	TelegramAuthDisabled   bool
	TelegramInitDataMaxAge time.Duration
	CORSAllowedOrigins     []string
	RateLimitPerMinute     int
	DatabaseURL            string
	UploadDir              string
	DataDir                string
	PythonBaseURL          string
	AdminUser              string
	AdminPassword          string
	AdminSecret            string
	VerifyClaimsEnabled    bool
}

var config *Config

// Loads .env and builds Config from environment variables.
func loadConfig() *Config {
	// .env is optional: absence is the normal case in Docker.
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	maxAgeSec, _ := strconv.Atoi(getEnv("TELEGRAM_INIT_DATA_MAX_AGE_SEC", "86400"))
	if maxAgeSec < 0 {
		maxAgeSec = 86400
	}
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT_REQUESTS_PER_MINUTE", "30"))
	if rateLimit < 0 {
		rateLimit = 0
	}

	return &Config{
		PythonServiceURL: getEnv("CLASSIFIER_URL", "http://classifier:5000/classify"),
		PythonRAGURL:     getEnv("CLASSIFIER_RAG_URL", "http://classifier:5000/rag/context"),
		LLMAPIKey:        getEnv("LLM_API_KEY", ""),
		LLMBaseURL:       getEnv("LLM_BASE_URL", "https://openrouter.ai/api"),
		LLMModel:         getEnv("LLM_MODEL", "google/gemini-2.5-flash-lite"),
		ServerPort:       getEnv("SERVER_PORT", "8080"),

		TelegramBotToken:       getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramAuthDisabled:   strings.EqualFold(getEnv("TELEGRAM_AUTH_DISABLED", "false"), "true"),
		TelegramInitDataMaxAge: time.Duration(maxAgeSec) * time.Second,
		CORSAllowedOrigins:     parseAllowedOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		RateLimitPerMinute:     rateLimit,
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://gardener:gardener@postgres:5432/gardener?sslmode=disable"),
		UploadDir:              getEnv("UPLOAD_DIR", "/data/uploads"),
		DataDir:                getEnv("DATA_DIR", "/app/data"),
		PythonBaseURL:          getEnv("PYTHON_BASE_URL", "http://classifier:5000"),
		AdminUser:              getEnv("ADMIN_USER", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", ""),
		AdminSecret:            getEnv("ADMIN_SECRET", ""),
		VerifyClaimsEnabled:    strings.EqualFold(getEnv("RAG_VERIFY_CLAIMS_ENABLED", "false"), "true"),
	}
}

// Returns an environment variable or defaultValue.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Logs main settings at server startup.
func logStartup(cfg *Config) {
	log.Printf("Starting Apple Gardener Server...")
	log.Printf("Python Classify URL: %s", cfg.PythonServiceURL)
	log.Printf("Python RAG context URL: %s", cfg.PythonRAGURL)
	log.Printf("LLM Model: %s", cfg.LLMModel)
	if cfg.LLMAPIKey != "" {
		log.Printf("LLM API Key: configured")
	} else {
		log.Printf("LLM API Key: not configured (using template responses)")
	}
	if cfg.TelegramAuthDisabled {
		log.Printf("Telegram auth: DISABLED (dev mode only)")
	} else if cfg.TelegramBotToken != "" {
		log.Printf("Telegram auth: enabled")
	} else {
		log.Printf("Telegram auth: WARNING — TELEGRAM_BOT_TOKEN not set, protected routes will reject clients")
	}
	if n := apiKeyCount(); n > 0 {
		log.Printf("API key auth: enabled (%d key(s))", n)
	} else {
		log.Printf("API key auth: not configured (set API_KEYS or API_KEYS_FILE for browser clients)")
	}
	log.Printf("CORS origins: %v", cfg.CORSAllowedOrigins)
	log.Printf("Rate limit: %d req/min per user", cfg.RateLimitPerMinute)
	if cfg.VerifyClaimsEnabled {
		if cfg.LLMAPIKey != "" {
			log.Printf("RAG claim verification: enabled (LLM judge)")
		} else {
			log.Printf("RAG claim verification: requested but disabled (no LLM_API_KEY)")
		}
	}
	log.Printf("Database URL: configured")
	log.Printf("Upload dir: %s", cfg.UploadDir)
	warnInsecureStartup(cfg)
}

// Warns about insecure defaults (does not block startup — dev only).
func warnInsecureStartup(cfg *Config) {
	if cfg.TelegramAuthDisabled {
		log.Printf("SECURITY: TELEGRAM_AUTH_DISABLED=true — local development only")
	}
	if cfg.TelegramBotToken == "" && !cfg.TelegramAuthDisabled {
		log.Printf("SECURITY: set TELEGRAM_BOT_TOKEN for production")
	}
	if cfg.AdminPassword == "" {
		log.Printf("SECURITY: ADMIN_PASSWORD not set — /api/admin/* disabled")
	}
	if cfg.AdminSecret == "" {
		log.Printf("SECURITY: ADMIN_SECRET not set — POST /admin/reindex disabled")
	}
	if strings.Contains(cfg.DatabaseURL, "gardener:gardener@") {
		log.Printf("SECURITY: change POSTGRES_PASSWORD (default gardener/gardener)")
	}
	if strings.Contains(cfg.LLMModel, ":free") {
		log.Printf("SECURITY: LLM_MODEL=%s — free-tier model is unstable for production", cfg.LLMModel)
	}
}
