package domain

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
)

type Database interface {
	Auth() AuthDatabase
	Pasta() PastaDatabase
}

type AuthDatabase interface {
	CreateUser(ctx context.Context, user *dto.RequestNewUser) error
	GetHashPassword(ctx context.Context, email string) (string, error)
	GetUserIDByEmail(ctx context.Context, email string) (int, error)
}

type PastaDatabase interface {
	CreateMetadata(ctx context.Context, pasta *models.Paste) error

	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)
	GetMetadata(ctx context.Context, pasta *models.Paste) error
	GetPassword(ctx context.Context, hash string) (string, error)
	GetKeys(ctx context.Context, userID int) ([]string, error)
	GetKeysExpiredPasta(ctx context.Context) ([]string, error)

	AddViews(ctx context.Context, hash string) error
	CheckPermission(ctx context.Context, userID int, hash string) (bool, error)
	DeleteMetadata(ctx context.Context, hash string) (string, error)
	DeleteExpiredPasta(ctx context.Context, keys []string) error
}
