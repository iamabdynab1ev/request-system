package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	AppEnv          string        `env:"APP_ENV" envDefault:"development"`
	ServerPort      string        `env:"SERVER_PORT" envDefault:"8080"`
	DatabaseURL     string        `env:"DATABASE_URL,required"`
	JWTSecret       string        `env:"JWT_SECRET_KEY,required"`
	AccessTokenTTL  time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"1h"`
	RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"168h"`
}

func LoadConfig(logger *zap.Logger) (*Config, error) {
	if err := godotenv.Load(); err != nil {
		logger.Warn("Warning: .env file not found, loading config from environment variables")
	}

	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config from environment: %w", err)
	}

	if cfg.AppEnv != "development" && len(cfg.JWTSecret) < 64 {
		logger.Warn("JWT_SECRET_KEY is too short for production environment")
	}

	return cfg, nil
}
