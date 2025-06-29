package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/internal/domain"
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

func (r *pastaCache) Views(ctx context.Context, hash string) (int, error) {
	keyViews := fmt.Sprintf("mets:%s:views", hash)
	views, err := r.redis.Incr(ctx, keyViews).Result()
	if err != nil {
		return 0, err
	}
	return int(views), nil
}

func (r *pastaCache) AddText(ctx context.Context, hash string, data []byte) error {

	err := r.redis.Set(ctx, "data:"+hash, data, 10*time.Second).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *pastaCache) AddMeta(ctx context.Context, pasta *models.Pasta) error {
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

func (r *pastaCache) GetText(ctx context.Context, keyData string) (*string, error) {
	resultText, err := r.redis.Get(ctx, keyData).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, customerrors.ErrKeyDoesntExist
		} else {
			return nil, err
		}
	}
	log.Println("текст из redis")
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
	log.Println("метаданые из redis")

	return &metadata, nil
}
