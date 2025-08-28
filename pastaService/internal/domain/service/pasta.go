package domain

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
)

//go:generate mockgen -source=pasta.go -destination=../mocks/service/pasta.go -package=mocks

type Pasta interface {
	Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error)
	Permission(ctx context.Context, hash, password, visibility string, userID int, forAccessCheck bool) error

	Get(ctx context.Context, hash string, withMetadata, addView bool) (*models.PastaWithData, error)
	GetText(ctx context.Context, keyText, objectID, hash string) (string, error)
	GetMetadata(ctx context.Context, keyMeta string, objectID string) (*models.Pasta, error)

	Delete(ctx context.Context, hash string) error
	Paginate(ctx context.Context, rawLimit, rawPage string, userID int, hasMetadata bool, paginateFn models.PaginateFunc) (*dto.PaginatedPastaDTO, error)
	Search(ctx context.Context, word string) ([]string, error)
	Update(ctx context.Context, newText []byte, hash string) (*models.Pasta, error)

	PaginateFavorite(ctx context.Context, rawLimit, rawPage string, userID int, hasMetadata bool) (*dto.PaginatedPastaDTO, error)
	PaginateOnlyPublic(ctx context.Context, rawLimit, rawPage string, hasMetadata bool) (*dto.PaginatedPastaDTO, error)
	PaginateForUserByID(ctx context.Context, rawLimit, rawPage string, userID int, hasMetadata bool) (*dto.PaginatedPastaDTO, error)

	Favorite(ctx context.Context, hash string, userID int) error
	GetFavorite(ctx context.Context, userID, favoriteID int, withMetadata bool) (*models.PastaWithData, error)
	DeleteFavorite(ctx context.Context, userID, favoriteID int) error

	GetVisibility(ctx context.Context, hash string) (string, error)

	GetExpiredPastas(ctx context.Context) ([]string, error)
	DeletePastas(ctx context.Context, objectIDs []string) error
}
