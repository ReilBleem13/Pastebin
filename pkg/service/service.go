package service

import (
	"pastebin/pkg/helpers"
	"pastebin/pkg/repository/database"
	"pastebin/pkg/repository/minio"
)

type Authorization interface{}

type Minio interface {
	CreateOne(objectID string, file helpers.FileDataType) (string, error)
	CreateMany(files map[string]helpers.FileDataType) ([]string, error)
	GetOne(objectID string) (string, error)
	GetMany(objectIDs []string) ([]string, error)
	DeleteOne(objectID string) error
	DeleteMany(objectIDs []string) error
}

type DBMinio interface {
	GetLink(objectID string) (string, error)
	CreateLink(objectID, url string) error
}

type Service struct {
	Authorization
	Minio
	DBMinio
}

func NewService(repo *database.Repository, minio minio.Client) *Service {
	return &Service{
		Authorization: NewAuthService(repo.Authorization),
		Minio:         NewMinioService(minio),
		DBMinio:       NewDBMinioService(repo.Minio),
	}
}
