package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port        string
	Env         string
	LogLevel    string

	DBUser     string
	DBPass     string
	DBHost     string
	DBPort     string
	DBName     string
	DBMaxOpen  int
	DBMaxIdle  int
	DBMaxLife  time.Duration

	JWTSecret  string
	JWTAccess  time.Duration
	JWTRefresh time.Duration

	SiteTimezone string

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load() *Config {
	return &Config{
		Port:       getEnv("PORT", "8080"),
		Env:        getEnv("ENV", "development"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),

		DBUser:     getEnv("DB_USER", "parking_user"),
		DBPass:     getEnv("DB_PASS", "parking_pass"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", "parking_db"),
		DBMaxOpen:  getEnvInt("DB_MAX_OPEN", 25),
		DBMaxIdle:  getEnvInt("DB_MAX_IDLE", 25),
		DBMaxLife:  getEnvDuration("DB_MAX_LIFE", "5m"),

		JWTSecret:  getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTAccess:  getEnvDuration("JWT_ACCESS", "15m"),
		JWTRefresh: getEnvDuration("JWT_REFRESH", "72h"),

		SiteTimezone: getEnv("SITE_TIMEZONE", "America/Bogota"),

		ReadTimeout:  getEnvDuration("READ_TIMEOUT", "10s"),
		WriteTimeout: getEnvDuration("WRITE_TIMEOUT", "10s"),
		IdleTimeout:  getEnvDuration("IDLE_TIMEOUT", "60s"),
	}
}

func (c *Config) Validate() error {
	if c.JWTSecret == "" || c.JWTSecret == "dev-secret-change-me" && c.Env == "production" {
		return fmt.Errorf("JWT_SECRET debe configurarse en producción")
	}
	if c.DBUser == "" || c.DBPass == "" || c.DBName == "" {
		return fmt.Errorf("DB_USER, DB_PASS y DB_NAME son requeridos")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		var n int
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return fallback
}

func getEnvDuration(key, fallback string) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	d, _ := time.ParseDuration(fallback)
	return d
}
