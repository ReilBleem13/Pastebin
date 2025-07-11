package domain

import (
	"context"
	"pastebin/internal/models"
)

//go:generate mockgen -source=database.go -destination=../mocks/repository/database.go -package=mocks

type Database interface {
	Pasta() PastaDatabase
}

type PastaDatabase interface {
	Create(ctx context.Context, pasta *models.Pasta) error

	GetHash(ctx context.Context, objectID string) (string, error)
	GetPublicHashs(ctx context.Context, objectID []string) ([]string, error)
	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)

	GetMetadata(ctx context.Context, objectID string) (*models.Pasta, error)
	GetManyMetadataPublic(ctx context.Context, objectID *[]string) (*[]models.Pasta, error)
	GetManyMetadataByUserID(ctx context.Context, objectID *[]string, userID int) (*[]models.Pasta, error)

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

	Paginate(ctx context.Context, limit, offset int) (*[]string, error)
	PaginateByUserID(ctx context.Context, limit, offset, userID int) (*[]string, error)
}

type ScannerDatabase interface {
	GetExpiredPastas(ctx context.Context) ([]string, error)
}
