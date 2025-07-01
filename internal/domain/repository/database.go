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
	Create(ctx context.Context, pasta *models.Pasta) error

	GetHash(ctx context.Context, objectID string) (string, error)
	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)

	GetMetadata(ctx context.Context, objectID string) (*models.Pasta, error)
	GetManyMetadata(ctx context.Context, objectID *[]string) (*[]models.Pasta, error)

	GetPassword(ctx context.Context, hash string) (string, error)
	GetKeys(ctx context.Context, userID int) ([]string, error)
	GetKeysExpiredPasta(ctx context.Context) ([]string, error)

	AddViews(ctx context.Context, hash string) error
	CheckPermission(ctx context.Context, userID int, hash string) (bool, error)
	DeleteMetadata(ctx context.Context, hash string) (string, error)
	DeleteExpiredPasta(ctx context.Context, keys []string) error

	IsPastaExists(ctx context.Context, hash string) (bool, error)
	IsPastaExistsByObjectID(ctx context.Context, objectID string) (bool, error)
	IsAccessPrivate(ctx context.Context, userID int, hash string) (bool, error)

	PaginateV1(ctx context.Context, limit, offset int) (*[]string, error)
}
