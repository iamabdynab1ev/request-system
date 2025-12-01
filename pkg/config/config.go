package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type FrontendConfig struct {
	BaseURL string
}
type OnlineBankConfig struct {
	BaseURL  string
	Username string
	Password string
}

type ProviderConfigs struct {
	OnlineBank OnlineBankConfig
}

type IntegrationsConfig struct {
	ActiveProvider         string
	Providers              ProviderConfigs
	OneCApiKey             string
	DefaultRolesFor1CUsers []string
}
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
	Port           string
	BaseURL        string
	AllowedOrigins []string
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Address  string
	Password string
}
type TelegramConfig struct {
	BotToken     string
	AdvancedMode bool
}
type NotificationConfig struct {
	Enabled              bool
	OverdueCheckInterval int
	ReminderThreshold    int
}

type LDAPConfig struct {
	Enabled bool
	Host    string
	Port    int
	Domain  string
}
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
}

func New() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Предупреждение: .env файл не найден или не удалось его загрузить.")
	}
	ldapPort, err := strconv.Atoi(getEnv("LDAP_PORT", "389"))
	if err != nil {
		log.Printf("Предупреждение: неверный формат LDAP_PORT, используется значение по умолчанию 389. Ошибка: %v", err)
		ldapPort = 389
	}
	telegramAdvancedMode := strings.ToLower(getEnv("TELEGRAM_ADVANCED_MODE_ENABLED", "false")) == "true"
	ldapEnabled := strings.ToLower(getEnv("LDAP_ENABLED", "false")) == "true"
	return &Config{
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			BaseURL:        getEnv("SERVER_BASE_URL", ""),
			AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:4040"), ","),
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
		Integrations: IntegrationsConfig{
			ActiveProvider:         getEnv("INTEGRATION_ACTIVE_PROVIDER", "mock"),
			OneCApiKey:             getEnv("ONE_C_API_KEY", ""),
			DefaultRolesFor1CUsers: strings.Split(getEnv("DEFAULT_ROLES_FOR_1C_USERS", ""), ","),
			Providers: ProviderConfigs{
				OnlineBank: OnlineBankConfig{
					BaseURL:  getEnv("ONLINEBANK_BASE_URL", ""),
					Username: getEnv("ONLINEBANK_USERNAME", ""),
					Password: getEnv("ONLINEBANK_PASSWORD", ""),
				},
			},
		},
		Telegram: TelegramConfig{
			BotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
			AdvancedMode: telegramAdvancedMode,
		},
		Frontend: FrontendConfig{
			BaseURL: getEnv("FRONTEND_BASE_URL", ""),
		},
		LDAP: LDAPConfig{
			Enabled: ldapEnabled,
			Host:    getEnv("LDAP_HOST", "arvand.local"),
			Port:    ldapPort,
			Domain:  getEnv("LDAP_DOMAIN", "arvand"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
