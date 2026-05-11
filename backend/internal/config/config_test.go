package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {

	t.Run("JWTSecretRequired", func(t *testing.T) {
		t.Parallel()

		// Clear JWT_SECRET so Load() panics.
		orig := os.Getenv("JWT_SECRET")
		os.Unsetenv("JWT_SECRET")
		defer os.Setenv("JWT_SECRET", orig)

		defer func() {
			r := recover()
			require.NotNil(t, r, "expected panic when JWT_SECRET is not set")
			assert.Contains(t, r.(string), "JWT_SECRET")
		}()
		Load()
	})

	t.Run("DefaultValues", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "test-secret-for-defaults")
		// Clear vars that have defaults so we test fallback.
		t.Setenv("SERVER_PORT", "")
		t.Setenv("LOG_LEVEL", "")
		t.Setenv("TOKEN_TYPE", "")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")
		t.Setenv("COOKIE_SECURE", "")

		cfg := Load()

		assert.Equal(t, "test-secret-for-defaults", cfg.JWTSecret)
		assert.Equal(t, "8080", cfg.ServerPort)
		assert.Equal(t, "info", cfg.LogLevel)
		assert.Equal(t, "Bearer", cfg.TokenType)
		assert.True(t, cfg.CookieSecure)
		assert.Equal(t, "", cfg.CORSAllowedOrigins)
		assert.Empty(t, cfg.DatabaseURL)
	})

	t.Run("EnvOverrides", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "my-super-secret")
		t.Setenv("SERVER_PORT", "9090")
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
		t.Setenv("LOG_LEVEL", "debug")
		t.Setenv("TOKEN_TYPE", "Custom")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,https://app.example.com")
		t.Setenv("COOKIE_SECURE", "false")

		cfg := Load()

		assert.Equal(t, "my-super-secret", cfg.JWTSecret)
		assert.Equal(t, "9090", cfg.ServerPort)
		assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", cfg.DatabaseURL)
		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, "Custom", cfg.TokenType)
		assert.False(t, cfg.CookieSecure)
	})
}

func TestConfig_Methods(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		ServerPort: "3000",
		JWTSecret:  "secret",
	}

	t.Run("ServerAddr", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, ":3000", cfg.ServerAddr())
	})

	t.Run("Timeouts", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, int64(15*1000_000_000), cfg.ReadTimeout().Nanoseconds())
		assert.Equal(t, int64(15*1000_000_000), cfg.WriteTimeout().Nanoseconds())
		assert.Equal(t, int64(30*1000_000_000), cfg.ShutdownTimeout().Nanoseconds())
	})

	t.Run("AllowedOrigins_Empty", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{CORSAllowedOrigins: ""}
		assert.Nil(t, cfg.AllowedOrigins())
	})

	t.Run("AllowedOrigins_Multiple", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{CORSAllowedOrigins: "https://a.com,https://b.com"}
		assert.Equal(t, []string{"https://a.com", "https://b.com"}, cfg.AllowedOrigins())
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("ReturnsValue", func(t *testing.T) {
		t.Setenv("TEST_GETENV_KEY", "hello")
		assert.Equal(t, "hello", GetEnv("TEST_GETENV_KEY", "fallback"))
	})

	t.Run("ReturnsFallback", func(t *testing.T) {
		assert.Equal(t, "fallback", GetEnv("TEST_GETENV_MISSING_KEY", "fallback"))
	})
}
