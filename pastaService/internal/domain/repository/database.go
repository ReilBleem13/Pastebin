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

	GetExpireAfterReadField(ctx context.Context, hash string) (bool, error)
	GetHash(ctx context.Context, objectID string) (string, error)
	GetPublicHashs(ctx context.Context, objectID []string) ([]string, error)
	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)
	GetUserID(ctx context.Context, hash string) (int, error)

	GetMetadata(ctx context.Context, objectID string) (*models.Pasta, error)
	GetManyMetadata(ctx context.Context, objectID []string) ([]models.Pasta, error)
	GetManyMetadataPublic(ctx context.Context, objectID []string) ([]models.Pasta, error)

	GetPassword(ctx context.Context, hash string) (string, error)
	GetKeys(ctx context.Context, userID int) ([]string, error)
	GetKeysExpiredPasta(ctx context.Context) ([]string, error)

	AddViews(ctx context.Context, hash string) error
	CheckPermission(ctx context.Context, userID int, hash string) (bool, error)
	DeleteMetadata(ctx context.Context, hash string) (string, error)
	DeleteExpiredPasta(ctx context.Context, keys []string) error

	IsPastaExists(ctx context.Context, hash string) (bool, error)
	IsPastaExistsByObjectID(ctx context.Context, objectID string) (bool, error)
	IsAccessPermission(ctx context.Context, userID int, hash string) (bool, error)

	PaginateOnlyPublic(ctx context.Context, limit, offset int) ([]string, error)
	PaginateOnlyByUserID(ctx context.Context, limit, offset, userID int) ([]string, error)
	PaginateFavorites(ctx context.Context, limit, offset, userID int) ([]string, error)

	Favorite(ctx context.Context, hash string, id int) error
	GetFavoriteAndCheckUser(ctx context.Context, userID, favoriteID int) (string, error)
	DeleteFavorite(ctx context.Context, userID, favoriteID int) error

	UpdateSizeAndReturnAll(ctx context.Context, hash string, size int) (*models.Pasta, error)

	GetExpiredPastas(ctx context.Context) ([]string, error)
	DeletePastas(ctx context.Context, objectIDs []string) error
}
