package domain

import (
	"context"
	"pastebin/internal/models"
)

//go:generate mockgen -source=cache.go -destination=../mocks/repository/cache.go -package=mocks

type Cache interface {
	Pasta() PastaCache
}

type PastaCache interface {
	AddText(ctx context.Context, hash string, data []byte) error
	AddMeta(ctx context.Context, pasta *models.Pasta) error
	GetText(ctx context.Context, keyData string) (*string, error)
	GetMeta(ctx context.Context, keyMeta string) (*models.Pasta, error)

	Views(ctx context.Context, hash string) (int, error)
	GetViews(ctx context.Context, hash string) (string, error)
}
