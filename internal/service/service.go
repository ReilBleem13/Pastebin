package service

import (
	"context"
	"pastebin/internal/models"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/minio"
	"pastebin/internal/repository/redis"
	"pastebin/pkg/dto"
	"pastebin/pkg/helpers"
)

type Authorization interface {
	CreateNewUser(user *dto.RequestNewUser) error
	CheckLogin(request *dto.LoginUser) error
	GenerateToken(request *dto.LoginUser) (string, error)
}

type Minio interface {
	CreateOne(ctx context.Context, data []byte) (models.Paste, error)
	CreateMany(files map[string]helpers.FileDataType) ([]string, error)
	GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error
	GetMany(objectIDs []string) ([]string, error)
	DeleteOne(hash string) error
	DeleteMany(objectIDs []string) error
	Test(maxKeys int, startAfter string)
}

type DBMinio interface {
	GetLink(hash string) (string, error)
	CreatePasta(request dto.RequestCreatePasta, pasta *models.Paste) error
	GetVisibility(hash string) (string, error)
	GetPastaByUserID(hash string) error
	AddViews(hash string) error

	CheckPublicPermission(hash string) (bool, error)
	CheckPastaPassword(password, hash string) error
	CheckPrivatePermission(userID int, hash string) (bool, error)
}

type Service struct {
	Authorization
	Minio
	DBMinio
}

func NewService(repo *database.Repository, minio minio.Client, redis redis.Redis) *Service {
	return &Service{
		Authorization: NewAuthService(repo.Authorization),
		Minio:         NewMinioService(minio, redis, repo.Minio),
		DBMinio:       NewDBMinioService(repo.Minio, redis),
	}
}
