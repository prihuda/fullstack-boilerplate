package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort        string
	DatabaseURL       string
	JWTSecret         string
	CORSAllowedOrigins string
	LogLevel          string
	CookieSecure      bool
	TokenType         string
}

func Load() *Config {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		panic("JWT_SECRET environment variable is required")
	}

	cookieSecure := true
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		cookieSecure, _ = strconv.ParseBool(v)
	}

	return &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         secret,
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		CookieSecure:      cookieSecure,
		TokenType:         getEnv("TOKEN_TYPE", "Bearer"),
	}
}

func (c *Config) ServerAddr() string {
	return fmt.Sprintf(":%s", c.ServerPort)
}

func (c *Config) ReadTimeout() time.Duration {
	return 15 * time.Second
}

func (c *Config) WriteTimeout() time.Duration {
	return 15 * time.Second
}

func (c *Config) ShutdownTimeout() time.Duration {
	return 30 * time.Second
}

func (c *Config) AllowedOrigins() []string {
	if c.CORSAllowedOrigins == "" {
		return nil
	}
	return strings.Split(c.CORSAllowedOrigins, ",")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
