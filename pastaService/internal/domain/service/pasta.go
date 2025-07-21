package domain

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
)

//go:generate mockgen -source=pasta.go -destination=../mocks/service/pasta.go -package=mocks

type Pasta interface {
	Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error)
	Permission(ctx context.Context, hash, password, visibility string, userID int) error

	Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error)
	GetText(ctx context.Context, keyText, objectID, hash string) (*string, error)
	GetMetadata(ctx context.Context, keyMeta string, objectID string) (*models.Pasta, error)

	Delete(ctx context.Context, hash string) error
	Paginate(ctx context.Context, rawLimit, rawPage string, hasMetadata bool, userID *int) (*dto.PaginatedPastaDTO, error)
	Search(ctx context.Context, word string) ([]string, error)
	Update(ctx context.Context, newText []byte, hash string) (*models.Pasta, error)

	GetVisibility(ctx context.Context, hash string) (string, error)
	GetUserID(ctx context.Context, hash string) (int, error)
}
