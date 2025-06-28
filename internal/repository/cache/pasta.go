package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/internal/domain"
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

func (r *pastaCache) AddMeta(ctx context.Context, pasta *models.Paste) error {
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

func (r *pastaCache) GetText(ctx context.Context, pasta *models.PasteWithData, keyData string) error {
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

func (r *pastaCache) GetMeta(ctx context.Context, pasta *models.PasteWithData, keyMeta string) error {
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
