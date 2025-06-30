package cache

import (
	domain "pastebin/internal/domain/repository"

	"github.com/go-redis/redis/v8"
)

type Cache struct {
	pasta domain.PastaCache
}

func NewCache(redis *redis.Client) *Cache {
	return &Cache{
		pasta: NewPastaCache(redis),
	}
}

func (c *Cache) Pasta() domain.PastaCache {
	return c.pasta
}
