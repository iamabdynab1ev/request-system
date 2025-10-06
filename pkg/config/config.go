package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type AuthConfig struct {
	ResetTokenTTL       time.Duration
	VerificationCodeTTL time.Duration
	MaxResetAttempts    int
	MaxLoginAttempts    int
	LockoutDuration     time.Duration
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type ServerConfig struct {
	Port string
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Address  string
	Password string
}

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Auth     AuthConfig
}

func New() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Предупреждение: .env файл не найден или не удалось его загрузить.")
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Postgres: PostgresConfig{
			DSN: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/request-system?sslmode=disable"),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			SecretKey:       getEnv("JWT_SECRET_KEY", "9A4D2AD385B2BAA8DC78F558B548F"),
			AccessTokenTTL:  time.Hour * 24,
			RefreshTokenTTL: time.Hour * 24 * 30,
		},
		Auth: AuthConfig{
			MaxResetAttempts:    5,
			MaxLoginAttempts:    5,
			LockoutDuration:     time.Minute * 15,
			ResetTokenTTL:       time.Minute * 15,
			VerificationCodeTTL: time.Minute * 15,
		},
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
