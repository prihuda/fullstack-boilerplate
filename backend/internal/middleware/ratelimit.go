package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultTrustedCIDRs are private/Docker IP ranges that bypass rate limiting.
var DefaultTrustedCIDRs = []string{
	"127.0.0.1/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"::1/128",
	"fc00::/7",
}

// RateLimiterConfig holds rate limiting configuration.
type RateLimiterConfig struct {
	Requests    int
	Window      time.Duration
	KeyPrefix   string
	TrustedCIDRs []string
}

// RateLimiter manages rate limiting via Redis.
type RateLimiter struct {
	client       *redis.Client
	config       RateLimiterConfig
	trustedCIDRs []*net.IPNet
}

// NewRateLimiter creates a new rate limiter backed by Redis.
func NewRateLimiter(client *redis.Client, config RateLimiterConfig) *RateLimiter {
	if config.KeyPrefix == "" {
		config.KeyPrefix = "ratelimit:"
	}

	cidrStrings := config.TrustedCIDRs
	if len(cidrStrings) == 0 {
		cidrStrings = DefaultTrustedCIDRs
	}
	var trustedCIDRs []*net.IPNet
	for _, c := range cidrStrings {
		_, cidr, err := net.ParseCIDR(c)
		if err == nil {
			trustedCIDRs = append(trustedCIDRs, cidr)
		}
	}

	return &RateLimiter{
		client:       client,
		config:       config,
		trustedCIDRs: trustedCIDRs,
	}
}

func (rl *RateLimiter) isTrustedIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range rl.trustedCIDRs {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// extractClientIP extracts the real client IP for rate limiting.
// Priority: CF-Connecting-IP (Cloudflare) > X-Real-IP (nginx) > RemoteAddr.
func extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware returns a chi middleware that rate limits requests by IP.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)

			if rl.isTrustedIP(ip) {
				next.ServeHTTP(w, r)
				return
			}

			key := rl.config.KeyPrefix + ip
			allowed, remaining, reset, err := rl.Allow(r.Context(), key)
			if err != nil {
				// If Redis fails, allow the request (fail-open)
				slog.Error("rate limiter failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.Requests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))

			if !allowed {
				WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Allow checks if a request is allowed under the rate limit using a sliding window.
// Only adds the request to the window if allowed (prevents memory leak from rejected requests).
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, int, time.Time, error) {
	now := time.Now()
	windowStart := now.Add(-rl.config.Window)

	pipe := rl.client.TxPipeline()

	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

	// Count current requests
	countCmd := pipe.ZCard(ctx, key)

	// Set expiry on the key
	pipe.Expire(ctx, key, rl.config.Window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, now, err
	}

	count := countCmd.Val()
	allowed := int(count) < rl.config.Requests

	if allowed {
		// Only add the request if allowed — prevents memory leak from rejected requests
		err = rl.client.ZAdd(ctx, key, redis.Z{
			Score:  float64(now.UnixNano()),
			Member: fmt.Sprintf("%d:%d", now.UnixNano(), now.UnixMilli()),
		}).Err()
		if err != nil {
			return false, 0, now, err
		}
	}

	remaining := rl.config.Requests - int(count) - 1
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, now.Add(rl.config.Window), nil
}

func DefaultRateLimitConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Requests:  300,
		Window:    1 * time.Minute,
		KeyPrefix: "ratelimit:",
	}
}

func AuthRateLimitConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Requests:  30,
		Window:    1 * time.Minute,
		KeyPrefix: "ratelimit-auth:",
	}
}
