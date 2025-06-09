package service

import (
	"context"
	"pastebin/internal/models"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/minio"
	"pastebin/internal/repository/redis"
	"pastebin/pkg/helpers"
)

type Authorization interface{}

type Minio interface {
	CreateOne(ctx context.Context, data []byte) (models.Paste, error)
	CreateMany(files map[string]helpers.FileDataType) ([]string, error)
	GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error
	GetMany(objectIDs []string) ([]string, error)
	DeleteOne(objectID string) error
	DeleteMany(objectIDs []string) error
}

type DBMinio interface {
	GetLink(hash string) (string, error)
	CreatePasta(pasta models.Paste) error
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
