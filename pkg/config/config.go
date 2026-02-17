package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server       ServerConfig
	Postgres     PostgresConfig
	Redis        RedisConfig
	JWT          JWTConfig
	Auth         AuthConfig
	Integrations IntegrationsConfig
	Telegram     TelegramConfig
	Frontend     FrontendConfig
	LDAP         LDAPConfig
	Seeder       SeederConfig
}

type ServerConfig struct {
	Port           string
	BaseURL        string
	AllowedOrigins []string
	CertFile       string
	KeyFile        string
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Address  string
	Password string
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type AuthConfig struct {
	ResetTokenTTL       time.Duration
	VerificationCodeTTL time.Duration
	MaxResetAttempts    int
	MaxLoginAttempts    int
	LockoutDuration     time.Duration
	SystemRootLogin     string
}

type IntegrationsConfig struct {
	ActiveProvider         string
	OneCApiKey             string
	DefaultRolesFor1CUsers []string
	OnlineBank             OnlineBankConfig
}

type OnlineBankConfig struct {
	BaseURL  string
	Username string
	Password string
}

type TelegramConfig struct {
	BotToken     string
	AdvancedMode bool
}

type FrontendConfig struct {
	BaseURL string
}

type LDAPConfig struct {
	Enabled bool
	Host    string
	Port    int
	Domain  string

	// Исправлено: Возвращаем пропущенное поле
	SearchEnabled bool

	BindDN              string
	BindPassword        string
	SearchBaseDN        string
	SearchFilterPattern string
	SearchAttributes    []string
	UsernameAttribute   string
	FIOAttribute        string
}

type SeederConfig struct {
	AdminEmail    string
	AdminPassword string
}

// New инициализирует конфигурацию, считывая .env
func New() *Config {
	// Если мы не в продакшене, пробуем загрузить .env, но не паникуем, если его нет
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Файл .env не найден или не может быть загружен. Используются системные переменные окружения.")
	} else {
		log.Println("✅ Файл .env загружен.")
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8091"),
			BaseURL:        getEnv("SERVER_BASE_URL", "https://localhost:8091"),
			AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ","),
			CertFile:       getEnv("SSL_CERT_PATH", "./certs/server.crt"),
			KeyFile:        getEnv("SSL_KEY_PATH", "./certs/server.key"),
		},
		Postgres: PostgresConfig{
			DSN: getRequiredEnv("DATABASE_URL"),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			SecretKey:       getRequiredEnv("JWT_SECRET_KEY"),
			AccessTokenTTL:  time.Hour * 24,
			RefreshTokenTTL: time.Hour * 24 * 30,
		},
		Auth: AuthConfig{
			MaxResetAttempts:    5,
			MaxLoginAttempts:    5,
			LockoutDuration:     15 * time.Minute,
			ResetTokenTTL:       15 * time.Minute,
			VerificationCodeTTL: 15 * time.Minute,
			SystemRootLogin:     strings.ToLower(getEnv("SEED_ADMIN_EMAIL", "admin@local")),
		},
		Seeder: SeederConfig{
			AdminEmail:    getEnv("SEED_ADMIN_EMAIL", ""),
			AdminPassword: getEnv("SEED_ADMIN_PASSWORD", ""),
		},
		Integrations: IntegrationsConfig{
			ActiveProvider:         getEnv("INTEGRATION_ACTIVE_PROVIDER", "mock"),
			OneCApiKey:             getEnv("ONE_C_API_KEY", ""),
			DefaultRolesFor1CUsers: parseList(getEnv("DEFAULT_ROLES_FOR_1C_USERS", "USER")),
			OnlineBank: OnlineBankConfig{
				BaseURL:  getEnv("ONLINEBANK_BASE_URL", ""),
				Username: getEnv("ONLINEBANK_USERNAME", ""),
				Password: getEnv("ONLINEBANK_PASSWORD", ""),
			},
		},
		Telegram: TelegramConfig{
			BotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
			AdvancedMode: getEnvAsBool("TELEGRAM_ADVANCED_MODE_ENABLED", false),
		},
		Frontend: FrontendConfig{
			BaseURL: getEnv("FRONTEND_BASE_URL", "http://localhost:3000"),
		},
		LDAP: LDAPConfig{
			Enabled:       getEnvAsBool("LDAP_ENABLED", false),
			SearchEnabled: getEnvAsBool("LDAP_SEARCH_ENABLED", false), // <-- Вернули чтение переменной
			Host:          getEnv("LDAP_HOST", "ldap.local"),
			Port:          getEnvAsInt("LDAP_PORT", 389),
			Domain:        getEnv("LDAP_DOMAIN", ""),
			BindDN:        getEnv("LDAP_BIND_DN", ""),
			BindPassword:  getEnv("LDAP_BIND_PASSWORD", ""),
			SearchBaseDN:  getEnv("LDAP_SEARCH_BASE_DN", ""),
			SearchFilterPattern: getEnv("LDAP_SEARCH_FILTER_PATTERN", "(&(objectClass=person)(sAMAccountName=%s))"),
			SearchAttributes:    parseList(getEnv("LDAP_SEARCH_ATTRIBUTES", "sAMAccountName,displayName,mail")),
			UsernameAttribute:   getEnv("LDAP_SEARCH_ATTR_USERNAME", "sAMAccountName"),
			FIOAttribute:        getEnv("LDAP_SEARCH_ATTR_FIO", "displayName"),
		},
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getRequiredEnv падает, если переменной нет (Fail Fast)
func getRequiredEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		log.Fatalf("❌ Критическая ошибка конфигурации: переменная окружения '%s' обязательна, но не установлена.", key)
	}
	return value
}

func getEnvAsBool(key string, fallback bool) bool {
	valStr := getEnv(key, "")
	if valStr == "" {
		return fallback
	}
	// true, TRUE, True, 1 -> true
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return fallback
	}
	return val
}

func getEnvAsInt(key string, fallback int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return fallback
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return fallback
	}
	return val
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
