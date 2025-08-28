package cache

import (
	domain "pastebin/internal/domain/repository"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	pasta domain.PastaCache
}

func NewCache(redis redis.UniversalClient) *Cache {
	return &Cache{
		pasta: NewPastaCache(redis),
	}
}

func (c *Cache) Pasta() domain.PastaCache {
	return c.pasta
}
