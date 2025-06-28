package cache

import (
	"context"
	"fmt"
	"pastebin/internal/config"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	redis *redis.Client
}

func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*RedisClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	redis := redis.NewClient(&redis.Options{
		Addr: cfg.Host,
	})

	if err := redis.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{redis: redis}, nil
}

func (r *RedisClient) Close() error {
	if r.redis == nil {
		return nil
	}
	return r.redis.Close()
}

func (r *RedisClient) Client() *redis.Client {
	return r.redis
}
