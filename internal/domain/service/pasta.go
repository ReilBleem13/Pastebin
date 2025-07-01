package domain

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
)

type Pasta interface {
	Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error)
	Permission(ctx context.Context, hash, password, visibility string, userID int) error
	Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error)
	GetText(ctx context.Context, keyText, objectID, hash string) (*string, error)
	Delete(ctx context.Context, hash string) error
	Paginate(ctx context.Context, rawLimit, startAfter string, userID *int) (*[]models.PastaPaginated, string, error)

	Paginate1(ctx context.Context, rawLimit, rawPage string, hasMetadata bool) (*[]dto.TextsWithMetadata, error)

	GetVisibility(ctx context.Context, hash string) (string, error) // временно

	// CreateOne(ctx context.Context, userID int, visibility, password *string, data []byte) (models.Pasta, error)
	// GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error
	// GetMany(ctx context.Context, objectIDs []string) ([]string, error)
	// DeleteOne(ctx context.Context, hash string) error
	// DeleteMany(ctx context.Context, objectIDs []string) error
	// Paginate(ctx context.Context, maxKeys, startAfter string, userID *int) ([]models.PastaPaginated, string, error)
}
