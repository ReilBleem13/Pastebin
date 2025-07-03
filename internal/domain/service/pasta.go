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

	Delete(ctx context.Context, hash string) error

	Paginate(ctx context.Context, rawLimit, rawPage string, hasMetadata bool, userID *int) (*[]dto.TextsWithMetadata, error)

	GetVisibility(ctx context.Context, hash string) (string, error) // временно
}
