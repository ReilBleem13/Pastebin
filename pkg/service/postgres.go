package service

import "pastebin/pkg/repository/database"

type DBMinioService struct {
	repo database.Minio
}

func NewDBMinioService(repo database.Minio) *DBMinioService {
	return &DBMinioService{repo: repo}
}

func (p *DBMinioService) CreateLink(objectID, url string) error {
	return p.repo.CreateLink(objectID, url)
}

func (p *DBMinioService) GetLink(objectID string) (string, error) {
	return p.repo.GetLink(objectID)
}
