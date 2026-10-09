package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the triage service components.
type Config struct {
	Environment       string        `json:"environment"`
	LogLevel          string        `json:"log_level"`
	HTTPAddr          string        `json:"http_addr"`
	MetricsAddr       string        `json:"metrics_addr"`
	DatabaseURL       string        `json:"database_url"`
	RedisURL          string        `json:"redis_url"`
	JWTSecret         string        `json:"-"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout"`
	WorkerConcurrency   int           `json:"worker_concurrency"`
	LLMProvider         string        `json:"llm_provider"`
	OpenAIBaseURL       string        `json:"openai_base_url"`
	OpenAIAPIKey        string        `json:"-"`
	OpenAIModel         string        `json:"openai_model"`
	GeminiBaseURL       string        `json:"gemini_base_url"`
	GeminiAPIKey        string        `json:"-"`
	GeminiModel         string        `json:"gemini_model"`
	StripeBaseURL       string        `json:"stripe_base_url"`
	StripeSecretKey     string        `json:"-"`
	StripeWebhookSecret string        `json:"-"`
	StripePublishableKey string       `json:"stripe_publishable_key"`
	StripeProPriceID    string        `json:"stripe_pro_price_id"`
	AppURL              string        `json:"app_url"`
}

// Load loads the configuration from system environment variables.
func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom loads and validates configuration using a custom lookup function.
func LoadFrom(lookup func(string) string) (Config, error) {
	geminiKey := lookup("GEMINI_API_KEY")
	openaiKey := lookup("OPENAI_API_KEY")
	defaultProvider := "mock"
	if geminiKey != "" {
		defaultProvider = "gemini"
	} else if openaiKey != "" {
		defaultProvider = "openai"
	}

	cfg := Config{
		Environment:          getEnvWithDefault(lookup, "APP_ENV", "development"),
		LogLevel:             getEnvWithDefault(lookup, "LOG_LEVEL", "info"),
		HTTPAddr:             getEnvWithDefault(lookup, "HTTP_ADDR", ":8080"),
		MetricsAddr:          getEnvWithDefault(lookup, "METRICS_ADDR", ":9090"),
		DatabaseURL:          lookup("DATABASE_URL"),
		RedisURL:             lookup("REDIS_URL"),
		JWTSecret:            getEnvWithDefault(lookup, "JWT_SECRET", "dev-secret-key-change-in-production-min32chars!"),
		ShutdownTimeout:      10 * time.Second,
		WorkerConcurrency:    5,
		LLMProvider:          getEnvWithDefault(lookup, "LLM_PROVIDER", defaultProvider),
		OpenAIBaseURL:        getEnvWithDefault(lookup, "OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIAPIKey:         openaiKey,
		OpenAIModel:          getEnvWithDefault(lookup, "OPENAI_MODEL", "gpt-4o"),
		GeminiBaseURL:        getEnvWithDefault(lookup, "GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		GeminiAPIKey:         geminiKey,
		GeminiModel:          getEnvWithDefault(lookup, "GEMINI_MODEL", "gemini-2.5-flash"),
		StripeBaseURL:        getEnvWithDefault(lookup, "STRIPE_BASE_URL", "https://api.stripe.com/v1"),
		StripeSecretKey:      lookup("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:  lookup("STRIPE_WEBHOOK_SECRET"),
		StripePublishableKey: lookup("STRIPE_PUBLISHABLE_KEY"),
		StripeProPriceID:     getEnvWithDefault(lookup, "STRIPE_PRO_PRICE_ID", "price_pro_monthly"),
		AppURL:               getEnvWithDefault(lookup, "APP_URL", "http://localhost:5173"),
	}

	if timeoutStr := lookup("SHUTDOWN_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.ShutdownTimeout = d
		}
	}

	if concurrencyStr := lookup("WORKER_CONCURRENCY"); concurrencyStr != "" {
		if c, err := strconv.Atoi(concurrencyStr); err == nil && c > 0 {
			cfg.WorkerConcurrency = c
		}
	}

	// Support PORT env var if HTTP_ADDR was not explicitly provided
	if lookup("HTTP_ADDR") == "" && lookup("PORT") != "" {
		port := lookup("PORT")
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		cfg.HTTPAddr = port
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate validates the configuration parameters.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	dbURL, err := url.Parse(c.DatabaseURL)
	if err != nil || (dbURL.Scheme != "postgres" && dbURL.Scheme != "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a valid postgres:// or postgresql:// URL, got: %s", c.DatabaseURL)
	}

	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}

	redisURL, err := url.Parse(c.RedisURL)
	if err != nil || (redisURL.Scheme != "redis" && redisURL.Scheme != "rediss") {
		return fmt.Errorf("REDIS_URL must be a valid redis:// or rediss:// URL, got: %s", c.RedisURL)
	}

	return nil
}

func getEnvWithDefault(lookup func(string) string, key, defaultVal string) string {
	val := lookup(key)
	if val == "" {
		return defaultVal
	}
	return val
}
