package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort   string
	DatabaseURL  string
	RedisURL     string
	NatsURL      string
	JWTSecret    string
	LogLevel     string
	Environment  string

	StripeSecretKey    string
	StripeWebhookSecret string
	BTCPayServerURL    string
	BTCPayAPIKey       string

	AgentAuthToken string

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func Load() *Config {
	return &Config{
		ServerPort:   envOrDefault("SERVER_PORT", "8080"),
		DatabaseURL:  envOrDefault("DATABASE_URL", "postgres://veritas:veritas_dev@localhost:5432/veritas?sslmode=disable"),
		RedisURL:     envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		NatsURL:      envOrDefault("NATS_URL", "nats://localhost:4222"),
		JWTSecret:    envRequired("JWT_SECRET"),
		LogLevel:     envOrDefault("LOG_LEVEL", "info"),
		Environment:  envOrDefault("ENVIRONMENT", "development"),

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		BTCPayServerURL:     os.Getenv("BTCPAY_SERVER_URL"),
		BTCPayAPIKey:        os.Getenv("BTCPAY_API_KEY"),

		AgentAuthToken: envRequired("AGENT_AUTH_TOKEN"),

		AccessTokenTTL:  durationEnvOrDefault("ACCESS_TOKEN_TTL", 1*time.Hour),
		RefreshTokenTTL: durationEnvOrDefault("REFRESH_TOKEN_TTL", 30*24*time.Hour),
	}
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envRequired(key string) string {
	val := os.Getenv(key)
	if val == "" {
		return val
	}
	return val
}

func durationEnvOrDefault(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development"
}

func (c *Config) ServerAddr() string {
	if strings.HasPrefix(c.ServerPort, ":") {
		return c.ServerPort
	}
	return ":" + c.ServerPort
}

func (c *Config) GRPCServerAddr() string {
	port, _ := strconv.Atoi(c.ServerPort)
	return ":" + strconv.Itoa(port+1000)
}
