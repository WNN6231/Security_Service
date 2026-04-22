package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const tokenBlacklistPrefix = "token:blacklist:"

type TokenBlacklist struct {
	rdb *redis.Client
}

func NewTokenBlacklist(rdb *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{rdb: rdb}
}

// Add puts a jti into the blacklist with the given TTL.
func (b *TokenBlacklist) Add(ctx context.Context, jti string, ttl time.Duration) error {
	key := tokenBlacklistPrefix + jti
	if err := b.rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("blacklist token: %w", err)
	}
	return nil
}

// IsBlacklisted checks whether a jti has been blacklisted.
func (b *TokenBlacklist) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := tokenBlacklistPrefix + jti
	n, err := b.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return n > 0, nil
}
