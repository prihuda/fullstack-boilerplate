package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db  Pinger
	rdb redis.Cmdable
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewHealthHandler(db Pinger, rdb redis.Cmdable) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Checks    struct {
		Database string `json:"database"`
		Redis    string `json:"redis"`
	} `json:"checks"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Check database
	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			resp.Checks.Database = "unhealthy"
			resp.Status = "degraded"
		} else {
			resp.Checks.Database = "healthy"
		}
	} else {
		resp.Checks.Database = "skipped"
	}

	// Check Redis
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			resp.Checks.Redis = "unhealthy"
			resp.Status = "degraded"
		} else {
			resp.Checks.Redis = "healthy"
		}
	} else {
		resp.Checks.Redis = "skipped"
	}

	status := http.StatusOK
	if resp.Status == "degraded" {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
