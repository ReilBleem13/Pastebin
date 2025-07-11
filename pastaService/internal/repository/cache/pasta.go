package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	domain "pastebin/internal/domain/repository"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"time"

	"github.com/go-redis/redis/v8"
)

type pastaCache struct {
	redis *redis.Client
}

func NewPastaCache(redis *redis.Client) domain.PastaCache {
	return &pastaCache{redis: redis}
}

const (
	textPrefix string = "text:"
	metaPrefix string = "meta:"

	viewsPrefix string = "views"

	textCacheTTL = 60 * time.Second // в конфиг
	metaCacheTTL = 60 * time.Second // в конфиг
)

func (r *pastaCache) Views(ctx context.Context, hash string, expiration *time.Duration) (int, error) {
	keyViews := fmt.Sprintf("%s:%s", viewsPrefix, hash)
	views, err := r.redis.Incr(ctx, keyViews).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to incr views: %w", err)
	}
	log.Printf("Просмотры: %d", views)

	if views == 1 {
		err = r.redis.Expire(ctx, keyViews, *expiration).Err()
		if err != nil {
			return 0, fmt.Errorf("failed to set deadline: %w", err)
		}
	}
	return int(views), nil
}

func (r *pastaCache) GetViews(ctx context.Context, hash string) (string, error) {
	keyViews := fmt.Sprintf("%s:%s", viewsPrefix, hash)
	result, err := r.redis.Get(ctx, keyViews).Result()
	if err != nil {
		if err == redis.Nil {
			return "", customerrors.ErrKeyDoesntExist
		} else {
			return "", err
		}
	}
	return result, nil
}

func (r *pastaCache) DeleteViews(ctx context.Context, hash string) error {
	keyViews := fmt.Sprintf("%s:%s", viewsPrefix, hash)
	if err := r.redis.Del(ctx, keyViews).Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("failed to delete key %s: %w", keyViews, err)
	}
	return nil
}

func (r *pastaCache) AddText(ctx context.Context, hash string, text []byte) error {
	return r.redis.Set(ctx, textPrefix+hash, text, textCacheTTL).Err()
}

func (r *pastaCache) AddMeta(ctx context.Context, pasta *models.Pasta) error {
	pastaJSON, err := json.Marshal(pasta)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, metaPrefix+pasta.Hash, pastaJSON, metaCacheTTL).Err()
}

func (r *pastaCache) GetText(ctx context.Context, keyText string) (*string, error) {
	resultText, err := r.redis.Get(ctx, keyText).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, customerrors.ErrKeyDoesntExist
		} else {
			return nil, err
		}
	}
	return &resultText, nil
}

func (r *pastaCache) GetMeta(ctx context.Context, keyMeta string) (*models.Pasta, error) {
	resultMeta, err := r.redis.Get(ctx, keyMeta).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, customerrors.ErrKeyDoesntExist
		} else {
			return nil, err
		}
	}

	metadata := models.Pasta{}
	if err := json.Unmarshal([]byte(resultMeta), &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}
