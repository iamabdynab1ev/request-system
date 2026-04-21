package postgresql

import (
	"context"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(dsn string) *pgxpool.Pool {
	log.Printf("ℹ️ Попытка подключения к БД для приложения: %s", sanitizeDSNForLog(dsn))

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Ошибка парсинга DSN: %v", err)
	}

	applyPoolConfig(poolConfig)

	dbpool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("Ошибка создания пула соединений к БД: %v", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Не удалось пинговать БД: %v", err)
	}

	log.Println("✅ Успешное подключение к PostgreSQL для приложения")
	log.Printf(
		"Пул PostgreSQL настроен: max_conns=%d, min_conns=%d, max_lifetime=%s, max_idle=%s, health_check=%s",
		poolConfig.MaxConns,
		poolConfig.MinConns,
		poolConfig.MaxConnLifetime,
		poolConfig.MaxConnIdleTime,
		poolConfig.HealthCheckPeriod,
	)

	return dbpool
}

func sanitizeDSNForLog(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "[dsn masked]"
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(username, "***")
		}
	}

	return parsed.String()
}

func applyPoolConfig(poolConfig *pgxpool.Config) {
	poolConfig.MaxConns = readEnvInt32("DB_POOL_MAX_CONNS", 30)
	poolConfig.MinConns = readEnvInt32("DB_POOL_MIN_CONNS", 5)
	poolConfig.MaxConnLifetime = readEnvDuration("DB_POOL_MAX_CONN_LIFETIME_MINUTES", 30*time.Minute, time.Minute)
	poolConfig.MaxConnIdleTime = readEnvDuration("DB_POOL_MAX_CONN_IDLE_MINUTES", 5*time.Minute, time.Minute)
	poolConfig.HealthCheckPeriod = readEnvDuration("DB_POOL_HEALTH_CHECK_PERIOD_SECONDS", 30*time.Second, time.Second)
}

func readEnvInt32(name string, fallback int32) int32 {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value <= 0 {
		return fallback
	}

	return int32(value)
}

func readEnvDuration(name string, fallback time.Duration, unit time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return time.Duration(value) * unit
}
