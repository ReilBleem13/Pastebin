package repository

import (
	"context"
	"pastebin/internal/models"
)

type MinioRepository interface {
	CreateMetadata(ctx context.Context, pasta *models.Paste) error

	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)
	GetMetadata(ctx context.Context, pasta *models.Paste) error
	GetPassword(ctx context.Context, hash string) (string, error)
	GetKeys(ctx context.Context, userID int) ([]string, error)

	AddViews(ctx context.Context, hash string) error
	CheckPermission(ctx context.Context, userID int, hash string) (bool, error)
	DeleteMetadata(ctx context.Context, hash string) (string, error)
}
