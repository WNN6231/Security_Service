package middleware

import (
	"fmt"
	"net/http"
	"time"

	"security-service/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RateLimitByIP enforces a sliding-window rate limit keyed by client IP.
func RateLimitByIP(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("ratelimit:ip:%s", clientIP(c))
		if !slidingWindowAllow(c, rdb, key, limit, window) {
			response.Error(c, http.StatusTooManyRequests, "too many requests, try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitByUser enforces a sliding-window rate limit keyed by authenticated user ID.
func RateLimitByUser(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		uidVal, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}
		key := fmt.Sprintf("ratelimit:user:%s", uidVal.(string))
		if !slidingWindowAllow(c, rdb, key, limit, window) {
			response.Error(c, http.StatusTooManyRequests, "too many requests, try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}

// slidingWindowAllow implements a Redis sorted-set sliding window counter.
func slidingWindowAllow(c *gin.Context, rdb *redis.Client, key string, limit int, window time.Duration) bool {
	ctx := c.Request.Context()
	now := time.Now()
	windowStart := now.Add(-window)

	// Remove expired entries and count remaining in one pipeline
	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return true // fail open on Redis error
	}

	if countCmd.Val() >= int64(limit) {
		return false
	}

	// Under limit — record this request
	member := uuid.New().String()
	rdb.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: member})
	rdb.Expire(ctx, key, window)
	return true
}
