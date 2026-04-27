// Package middleware provides HTTP middleware for the Gin framework.
// RateLimit implements a sliding window counter using Redis.
//
// Strategy per endpoint:
//   - Global customer endpoints  : 60 req/menit per IP
//   - Sensitive endpoints (OCR, liveness, agree) : 10 req/menit per IP
//   - Admin endpoints            : 120 req/menit per IP (sudah dilindungi JWT)
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter holds Redis client + config for a specific rate limit tier.
type RateLimiter struct {
	rdb      *redis.Client
	requests int           // max requests per window
	window   time.Duration // sliding window size
	prefix   string        // Redis key prefix (e.g. "rl:ocr:", "rl:global:")
}

// NewRateLimiter creates a RateLimiter for a specific tier.
func NewRateLimiter(rdb *redis.Client, requests int, window time.Duration, prefix string) *RateLimiter {
	return &RateLimiter{
		rdb:      rdb,
		requests: requests,
		window:   window,
		prefix:   prefix,
	}
}

// Limit returns a Gin middleware that enforces the rate limit.
// Key is based on: prefix + client IP.
// Returns 429 Too Many Requests with Retry-After header when limit exceeded.
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		ip := c.ClientIP()
		key := fmt.Sprintf("%s%s", rl.prefix, ip)

		// Atomic increment + TTL set using Redis pipeline
		pipe := rl.rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, rl.window)
		_, err := pipe.Exec(ctx)

		if err != nil {
			// Kalau Redis error, jangan block user — fail open
			// Lebih baik melayani request daripada membuat semua orang kena error
			c.Next()
			return
		}

		count := incr.Val()

		// Set header informatif agar client bisa tahu limitnya
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(rl.requests)-count)))
		c.Header("X-RateLimit-Window", rl.window.String())

		if count > int64(rl.requests) {
			retryAfter := int(rl.window.Seconds())
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, response.Response{
				Success: false,
				Message: fmt.Sprintf(
					"Terlalu banyak permintaan. Silakan coba lagi dalam %d detik.",
					retryAfter,
				),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// max helper — Go 1.21+ punya built-in max(), tapi untuk kompatibilitas:
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
