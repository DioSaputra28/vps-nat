package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Telegram TelegramConfig
	Pakasir  PakasirConfig
	Alerts   AlertConfig
	Incus    IncusConfig
	Caddy    CaddyConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Host string
	Port int
}

type AuthConfig struct {
	SessionTTL time.Duration
}

type TelegramConfig struct {
	BotSecret string
}

type PakasirConfig struct {
	BaseURL     string
	ProjectSlug string
	APIKey      string
}

type AlertConfig struct {
	TelegramBotToken string
	TelegramChatID   string
}

type DatabaseConfig struct {
	URL             string
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	TimeZone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type IncusConfig struct {
	Enabled            bool
	Mode               string
	Socket             string
	RemoteAddr         string
	NetworkName        string
	UserAgent          string
	TLSClientCertPath  string
	TLSClientKeyPath   string
	TLSCAPath          string
	TLSServerCertPath  string
	InsecureSkipVerify bool
}

type CaddyConfig struct {
	AdminURL string
	APIToken string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "vps-nat"),
			Env:  getEnv("APP_ENV", "development"),
		},
		HTTP: HTTPConfig{
			Host: getEnv("HTTP_HOST", "0.0.0.0"),
			Port: getEnvInt("HTTP_PORT", 8080),
		},
		Auth: AuthConfig{
			SessionTTL: getEnvDuration("AUTH_ADMIN_SESSION_TTL", 24*time.Hour),
		},
		Telegram: TelegramConfig{
			BotSecret: os.Getenv("TELEGRAM_BOT_SECRET"),
		},
		Pakasir: PakasirConfig{
			BaseURL:     getEnv("PAKASIR_BASE_URL", "https://app.pakasir.com"),
			ProjectSlug: os.Getenv("PAKASIR_PROJECT_SLUG"),
			APIKey:      os.Getenv("PAKASIR_API_KEY"),
		},
		Alerts: AlertConfig{
			TelegramBotToken: os.Getenv("ADMIN_ALERT_TELEGRAM_BOT_TOKEN"),
			TelegramChatID:   os.Getenv("ADMIN_ALERT_TELEGRAM_CHAT_ID"),
		},
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			Host:            getEnv("DB_HOST", "127.0.0.1"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        os.Getenv("DB_PASSWORD"),
			Name:            getEnv("DB_NAME", "vps_nat"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			TimeZone:        getEnv("DB_TIMEZONE", "UTC"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Incus: IncusConfig{
			Enabled:            getEnvBool("INCUS_ENABLED", false),
			Mode:               getEnv("INCUS_MODE", "unix"),
			Socket:             os.Getenv("INCUS_SOCKET"),
			RemoteAddr:         os.Getenv("INCUS_REMOTE_ADDR"),
			NetworkName:        os.Getenv("INCUS_NETWORK_NAME"),
			UserAgent:          getEnv("INCUS_USER_AGENT", "vps-nat-backend"),
			TLSClientCertPath:  os.Getenv("INCUS_TLS_CLIENT_CERT_PATH"),
			TLSClientKeyPath:   os.Getenv("INCUS_TLS_CLIENT_KEY_PATH"),
			TLSCAPath:          os.Getenv("INCUS_TLS_CA_PATH"),
			TLSServerCertPath:  os.Getenv("INCUS_TLS_SERVER_CERT_PATH"),
			InsecureSkipVerify: getEnvBool("INCUS_TLS_INSECURE_SKIP_VERIFY", false),
		},
		Caddy: CaddyConfig{
			AdminURL: os.Getenv("CADDY_ADMIN_URL"),
			APIToken: os.Getenv("CADDY_ADMIN_API_TOKEN"),
		},
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c DatabaseConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}

	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		c.Host,
		c.User,
		c.Password,
		c.Name,
		c.Port,
		c.SSLMode,
		c.TimeZone,
	)
}

func validate(cfg Config) error {
	if cfg.HTTP.Port <= 0 {
		return errors.New("HTTP_PORT must be greater than 0")
	}

	if cfg.Database.MaxOpenConns <= 0 {
		return errors.New("DB_MAX_OPEN_CONNS must be greater than 0")
	}

	if cfg.Database.MaxIdleConns < 0 {
		return errors.New("DB_MAX_IDLE_CONNS must be zero or greater")
	}

	if cfg.Auth.SessionTTL <= 0 {
		return errors.New("AUTH_ADMIN_SESSION_TTL must be greater than 0")
	}

	if !cfg.Incus.Enabled {
		return nil
	}

	switch cfg.Incus.Mode {
	case "unix":
		return nil
	case "remote":
		if cfg.Incus.RemoteAddr == "" {
			return errors.New("INCUS_REMOTE_ADDR is required when INCUS_MODE=remote")
		}
		if cfg.Incus.TLSClientCertPath == "" || cfg.Incus.TLSClientKeyPath == "" {
			return errors.New("INCUS_TLS_CLIENT_CERT_PATH and INCUS_TLS_CLIENT_KEY_PATH are required when INCUS_MODE=remote")
		}

		return nil
	default:
		return fmt.Errorf("unsupported INCUS_MODE %q", cfg.Incus.Mode)
	}
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return value
}
