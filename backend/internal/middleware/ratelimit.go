package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimitConfig struct {
	Requests  int
	Window    time.Duration
	KeyPrefix string
}

func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Requests:  300,
		Window:    1 * time.Minute,
		KeyPrefix: "rl:",
	}
}

// AuthRateLimitConfig returns a stricter rate limit config for auth routes.
func AuthRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Requests:  30,
		Window:    1 * time.Minute,
		KeyPrefix: "rl-auth:",
	}
}

var trustedCIDRs = []*net.IPNet{
	parseCIDR("127.0.0.1/8"),
	parseCIDR("10.0.0.0/8"),
	parseCIDR("172.16.0.0/12"),
	parseCIDR("192.168.0.0/16"),
	parseCIDR("::1/128"),
	parseCIDR("fc00::/7"),
}

func parseCIDR(s string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return cidr
}

func isTrustedIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range trustedCIDRs {
		if cidr != nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func extractIP(r *http.Request) string {
	// Priority: CF-Connecting-IP (Cloudflare) > X-Real-IP (nginx) > RemoteAddr
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

func RateLimiter(rdb redis.Cmdable, cfg RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)

			if isTrustedIP(ip) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			now := float64(time.Now().UnixMilli())
			windowStart := now - float64(cfg.Window.Milliseconds())
			key := cfg.KeyPrefix + ip

			// Step 1: Clean old entries outside the window
			zrem := rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%f", windowStart))
			if zrem.Err() != nil {
				slog.Error("rate limiter zrem failed", "error", zrem.Err())
				next.ServeHTTP(w, r)
				return
			}

			// Step 2: Count current entries in the window
			count, err := rdb.ZCard(ctx, key).Result()
			if err != nil {
				slog.Error("rate limiter zcard failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			// Step 3: Check if over limit BEFORE adding
			if count >= int64(cfg.Requests) {
				resetAt := time.Now().Add(cfg.Window).Unix()
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Requests))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
				retryAfter := cfg.Window.Seconds()
				w.Header().Set("Retry-After", strconv.FormatFloat(retryAfter, 'f', 0, 64))
				http.Error(w, `{"code":"RATE_LIMITED","message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}

			// Step 4: Under limit — add entry with unique member to avoid dedup
			member := fmt.Sprintf("%d:%f", time.Now().UnixNano(), now)
			if err := rdb.ZAdd(ctx, key, redis.Z{Score: now, Member: member}).Err(); err != nil {
				slog.Error("rate limiter zadd failed", "error", err)
			}

			remaining := cfg.Requests - int(count) - 1
			if remaining < 0 {
				remaining = 0
			}

			resetAt := time.Now().Add(cfg.Window).Unix()

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Requests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

			// Set expiry on the key
			rdb.Expire(ctx, key, cfg.Window+time.Second)

			next.ServeHTTP(w, r)
		})
	}
}
