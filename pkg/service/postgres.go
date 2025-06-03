package service

import "pastebin/pkg/repository/database"

type DBMinioService struct {
	repo database.Minio
}

func NewDBMinioService(repo database.Minio) *DBMinioService {
	return &DBMinioService{repo: repo}
}

func (p *DBMinioService) CreateLink(objectID, hash string) error {
	return p.repo.CreateLink(objectID, hash)
}

func (p *DBMinioService) GetLink(hash string) (string, error) {
	return p.repo.GetLink(hash)
}
