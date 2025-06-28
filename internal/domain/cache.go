package domain

import (
	"context"
	"pastebin/internal/models"
)

type Cache interface {
	Pasta() PastaCache
}

type PastaCache interface {
	AddText(ctx context.Context, hash string, data []byte) error
	AddMeta(ctx context.Context, pasta *models.Paste) error
	GetText(ctx context.Context, pasta *models.PasteWithData, keyData string) error
	GetMeta(ctx context.Context, pasta *models.PasteWithData, keyMeta string) error
	Views(ctx context.Context, hash string) (int, error)
}
