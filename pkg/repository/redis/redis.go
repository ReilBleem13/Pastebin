package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/pkg/models"
	"time"

	"github.com/go-redis/redis/v8"
)

type Redis interface {
	InitRedis() error
	Add(ctx context.Context, pasta *models.Paste, data []byte) error
	Get(ctx context.Context, pasta *models.PasteWithData, keyData, keyMeta string) error
}

type RedisClient struct {
	redis *redis.Client
}

func NewRedisClient() Redis {
	return &RedisClient{}
}

func (r *RedisClient) InitRedis() error {
	ctx := context.Background()
	redis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	if err := redis.Ping(ctx).Err(); err != nil {
		return err
	}

	r.redis = redis
	return nil
}

func (r *RedisClient) Add(ctx context.Context, pasta *models.Paste, data []byte) error {
	pastaJSON, err := json.Marshal(pasta)
	if err != nil {
		return err
	}

	err = r.redis.Set(ctx, "meta:"+pasta.Hash, pastaJSON, 24*time.Hour).Err()
	if err != nil {
		return err
	}

	err = r.redis.Set(ctx, "data:"+pasta.Hash, data, 1*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	return nil
}

func (r *RedisClient) Get(ctx context.Context, pasta *models.PasteWithData, keyData, keyMeta string) error {
	resultText, err := r.redis.Get(ctx, keyData).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key doesn't exists: %v", err)
		} else {
			return err
		}
	}
	pasta.Text = resultText

	log.Println("FROM REDIS")
	if keyMeta == "" {
		return nil
	}

	resultMeta, err := r.redis.Get(ctx, keyMeta).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key doesn't exists: %v", err)
		} else {
			return err
		}
	}

	var metadata models.Paste
	if err := json.Unmarshal([]byte(resultMeta), &metadata); err != nil {
		return err
	}

	pasta.Metadata = metadata
	log.Println("FROM REDIS")
	return nil
}
