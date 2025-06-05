package service

import (
	"pastebin/pkg/models"
	"pastebin/pkg/repository/database"
	"pastebin/pkg/repository/redis"
)

type DBMinioService struct {
	repo  database.Minio
	redis redis.Redis
}

func NewDBMinioService(repo database.Minio, redis redis.Redis) *DBMinioService {
	return &DBMinioService{repo: repo, redis: redis}
}

func (p *DBMinioService) CreatePasta(pasta models.Paste) error {
	return p.repo.CreatePasta(pasta)
}

func (p *DBMinioService) GetLink(hash string) (string, error) {
	return p.repo.GetLink(hash)
}
