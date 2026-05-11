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

// rateLimitScript is a Lua script for atomic check-and-add rate limiting.
var rateLimitScript = redis.NewScript(`
    redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[1])
    local count = redis.call('ZCARD', KEYS[1])
    if tonumber(count) < tonumber(ARGV[2]) then
        redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
        redis.call('EXPIRE', KEYS[1], ARGV[5])
        return {1, count + 1}
    end
    return {0, count}
`)

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
			allowed, count, err := rl.Allow(r.Context(), key)
			if err != nil {
				// If Redis fails, allow the request (fail-open)
				slog.Error("rate limiter failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			remaining := rl.config.Requests - count
			if remaining < 0 {
				remaining = 0
			}
			reset := time.Now().Add(rl.config.Window)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.Requests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))

			if !allowed {
				WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Allow checks if a request is allowed under the rate limit using an atomic Lua script.
// Returns whether the request is allowed, the current request count, and any error.
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	now := time.Now()
	member := strconv.FormatInt(now.UnixNano(), 10)
	cutoff := now.Add(-rl.config.Window).UnixMicro()
	ttl := int(rl.config.Window.Seconds()) * 2

	result, err := rateLimitScript.Run(ctx, rl.client, []string{key}, cutoff, rl.config.Requests, now.UnixMicro(), member, ttl).Result()
	if err != nil {
		return true, rl.config.Requests, err
	}

	vals, ok := result.([]any)
	if !ok || len(vals) < 2 {
		return true, rl.config.Requests, nil
	}

	allowed, _ := strconv.Atoi(fmt.Sprintf("%v", vals[0]))
	count, _ := strconv.Atoi(fmt.Sprintf("%v", vals[1]))

	return allowed == 1, count, nil
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
