package service

import (
	domain "pastebin/internal/domain/service"
	"pastebin/internal/infrastructure/kafka"
	"pastebin/internal/repository"
	"pastebin/pkg/logging"
)

// type Authorization interface {
// 	CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error
// 	CheckLogin(ctx context.Context, request *dto.LoginUser) error
// 	GenerateToken(ctx context.Context, request *dto.LoginUser) (string, error)
// }

// type Pasta interface {
// 	Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error)
// 	Permission(ctx context.Context, hash, password, visibility string, userID int) error
// 	Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error)
// 	GetText(ctx context.Context, keyText, objectID, hash string) (*string, error)

// 	GetVisibility(ctx context.Context, hash string) (string, error) // временно

// 	// CreateOne(ctx context.Context, userID int, visibility, password *string, data []byte) (models.Pasta, error)
// 	// GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error
// 	// GetMany(ctx context.Context, objectIDs []string) ([]string, error)
// 	// DeleteOne(ctx context.Context, hash string) error
// 	// DeleteMany(ctx context.Context, objectIDs []string) error
// 	// Paginate(ctx context.Context, maxKeys, startAfter string, userID *int) ([]models.PastaPaginated, string, error)
// }

// type DBMinio interface {
// 	GetLink(ctx context.Context, hash string) (string, error)
// 	CreatePasta(ctx context.Context, request dto.RequestCreatePasta, pasta *models.Pasta) error
// 	GetVisibility(ctx context.Context, hash string) (string, error)
// 	AddViews(ctx context.Context, hash string) error

// 	CheckPublicPermission(ctx context.Context, hash string) (bool, error)
// 	CheckPastaPassword(ctx context.Context, password, hash string) error
// 	CheckPrivatePermission(ctx context.Context, userID int, hash string) (bool, error)
// }

// type Cleanup interface {
// 	CleanupExpiredPasta(ctx context.Context) error
// }

type Service struct {
	Authorization domain.Authorization
	Pasta         domain.Pasta
	// DBMinio
	// Cleanup
}

func NewService(repo *repository.Repository, producer *kafka.Producer, logger *logging.Logger) *Service {
	return &Service{
		Authorization: NewAuthService(repo),
		Pasta:         NewPastaService(repo, producer, logger),
	}
}
