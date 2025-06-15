package service

import (
	"context"
	"pastebin/internal/domain/repository"
	"pastebin/internal/models"
	"pastebin/internal/repository/postgres"
	"pastebin/pkg/dto"
)

type Authorization interface {
	CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error
	CheckLogin(ctx context.Context, request *dto.LoginUser) error
	GenerateToken(ctx context.Context, request *dto.LoginUser) (string, error)
}

type Minio interface {
	CreateOne(ctx context.Context, userID int, visibility, password *string, data []byte) (models.Paste, error)
	GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error
	GetMany(ctx context.Context, objectIDs []string) ([]string, error)
	DeleteOne(ctx context.Context, hash string) error
	DeleteMany(ctx context.Context, objectIDs []string) error
	Paginate(ctx context.Context, maxKeys, startAfter string, userID *int) ([]models.PastaPaginated, string, error)
}

type DBMinio interface {
	GetLink(ctx context.Context, hash string) (string, error)
	CreatePasta(ctx context.Context, request dto.RequestCreatePasta, pasta *models.Paste) error
	GetVisibility(ctx context.Context, hash string) (string, error)
	AddViews(ctx context.Context, hash string) error

	CheckPublicPermission(ctx context.Context, hash string) (bool, error)
	CheckPastaPassword(ctx context.Context, password, hash string) error
	CheckPrivatePermission(ctx context.Context, userID int, hash string) (bool, error)
}

type Cleanup interface {
	CleanupExpiredPasta(ctx context.Context) error
}

type Service struct {
	Authorization
	Minio
	DBMinio
	Cleanup
}

func NewService(repo postgres.Repository, minio repository.FileRepository, redis repository.RedisRepository) *Service {
	return &Service{
		Authorization: NewAuthService(repo.Auth),
		Minio:         NewMinioService(minio, redis, repo.Minio),
		DBMinio:       NewDBMinioService(repo.Minio, redis),
		Cleanup:       NewExpiredDeleteService(repo.Minio, minio),
	}
}
