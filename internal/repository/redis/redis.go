package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/internal/models"
	"time"

	"github.com/go-redis/redis/v8"
)

type Redis interface {
	InitRedis() error
	AddText(ctx context.Context, hash string, data []byte) error
	AddMeta(ctx context.Context, pasta *models.Paste) error
	GetText(ctx context.Context, pasta *models.PasteWithData, keyData string) error
	GetMeta(ctx context.Context, pasta *models.PasteWithData, keyMeta string) error
	Views(ctx context.Context, hash string) (int, error)
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

func (r *RedisClient) Views(ctx context.Context, hash string) (int, error) {
	keyViews := fmt.Sprintf("mets:%s:views", hash)
	views, err := r.redis.Incr(ctx, keyViews).Result()
	if err != nil {
		return 0, err
	}
	return int(views), nil
}

func (r *RedisClient) AddText(ctx context.Context, hash string, data []byte) error {

	err := r.redis.Set(ctx, "data:"+hash, data, 10*time.Second).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) AddMeta(ctx context.Context, pasta *models.Paste) error {
	pastaJSON, err := json.Marshal(pasta)
	if err != nil {
		return err
	}

	err = r.redis.Set(ctx, "meta:"+pasta.Hash, pastaJSON, 60*time.Second).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) GetText(ctx context.Context, pasta *models.PasteWithData, keyData string) error {
	resultText, err := r.redis.Get(ctx, keyData).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key doesn't exists: %v", err)
		} else {
			return err
		}
	}
	pasta.Text = resultText
	log.Println("текст из redis")
	return nil
}

func (r *RedisClient) GetMeta(ctx context.Context, pasta *models.PasteWithData, keyMeta string) error {
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
	log.Println("метаданые из redis")

	return nil
}
