package domain

import (
	"context"
	"pastebin/internal/models"
	"time"
)

//go:generate mockgen -source=cache.go -destination=../mocks/repository/cache.go -package=mocks

type Cache interface {
	Pasta() PastaCache
}

type PastaCache interface {
	AddText(ctx context.Context, hash string, data []byte) error
	AddMeta(ctx context.Context, pasta *models.Pasta) error
	GetText(ctx context.Context, keyData string) (string, error)
	GetMeta(ctx context.Context, keyMeta string) (*models.Pasta, error)

	DeleteAll(ctx context.Context, hash string) error
	DeleteViews(ctx context.Context, hash string) error
	DeleteText(ctx context.Context, hash string) error
	DeleteMeta(ctx context.Context, hash string) error

	CreateViews(ctx context.Context, hash string, expiration *time.Duration) error
	IncrViews(ctx context.Context, hash string) (int, error)
	GetViews(ctx context.Context, hash string) (string, error)
}
