package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/docgen"
	"github.com/redis/go-redis/v9"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/config"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/handler"
	authmw "github.com/rhuda/fullstack-boilerplate/backend/internal/middleware"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/repository"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/service"
	"github.com/rhuda/fullstack-boilerplate/backend/pkg/database"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup structured logger
	logger := setupLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to PostgreSQL
	db, err := database.NewDB(ctx, database.DefaultPostgresConfig(cfg.DatabaseURL))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to database")

	// Run migrations
	if err := runMigrations(ctx, db.BunDB); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.GetEnv("KEYDB_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("redis connection failed — rate limiting disabled", "error", err)
	} else {
		slog.Info("connected to redis")
	}
	defer rdb.Close()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.BunDB)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db.BunDB)

	// Background context for goroutines — cancelled on shutdown
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Periodic refresh token cleanup (expired + revoked)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		defer func() {
			if err := recover(); err != nil {
				slog.Error("cleanup goroutine panicked", "error", err)
			}
		}()
		for {
			select {
			case <-bgCtx.Done():
				slog.Info("cleanup goroutine stopped")
				return
			case <-ticker.C:
				cleanupCtx, cleanupCancel := context.WithTimeout(bgCtx, 30*time.Second)
				if err := refreshTokenRepo.DeleteExpiredAndRevoked(cleanupCtx, db.BunDB); err != nil {
					slog.Warn("failed to clean up expired tokens", "error", err)
				}
				cleanupCancel()
			}
		}
	}()

	// Initialize services
	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg.JWTSecret)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService, cfg)
	healthHandler := handler.NewHealthHandler(db, rdb)

	// Build router
	r := chi.NewRouter()

	// Global middleware — RequestID/RealIP/CleanPath first for downstream middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.CleanPath)
	r.Use(authmw.Recover(logger))
	r.Use(authmw.Logger(logger))
	r.Use(authmw.SecurityHeaders())
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Global rate limiter (Redis-backed)
	if rdb.Ping(ctx).Err() == nil {
		rlCfg := authmw.DefaultRateLimitConfig()
		r.Use(authmw.NewRateLimiter(rdb, rlCfg).Middleware())
	}

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			// Stricter rate limit for auth routes
			if rdb.Ping(ctx).Err() == nil {
				authRLCfg := authmw.AuthRateLimitConfig()
				r.Use(authmw.NewRateLimiter(rdb, authRLCfg).Middleware())
			}
			r.Mount("/", authHandler.Routes())
		})
		r.Get("/health", healthHandler.ServeHTTP)
	})

	// Pre-compute API docs before registering the endpoint (avoids self-referencing)
	jsonDocs := docgen.JSONRoutesDoc(r)

	r.Get("/docs/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonDocs))
	})

	// Custom 404/405 handlers for consistent JSON API responses
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		authmw.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "The requested resource was not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		authmw.WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:              cfg.ServerAddr(),
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout(),
		WriteTimeout:      cfg.WriteTimeout(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down server", "signal", sig.String())

	// Stop background goroutines
	bgCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}

	slog.Info("server stopped")
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})
	return slog.New(handler)
}
