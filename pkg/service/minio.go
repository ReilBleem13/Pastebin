package service

import (
	"pastebin/pkg/helpers"
	"pastebin/pkg/repository/minio"
)

type MinioService struct {
	client minio.Client
}

func NewMinioService(minio minio.Client) *MinioService {
	return &MinioService{
		client: minio,
	}
}

func (m *MinioService) CreateOne(objectID string, file helpers.FileDataType) (string, error) {
	return m.client.CreateOne(objectID, file)
}

func (m *MinioService) CreateMany(files map[string]helpers.FileDataType) ([]string, error) {
	return m.client.CreateMany(files)
}

func (m *MinioService) GetOne(objectID string) (string, error) {
	return m.client.GetOne(objectID)
}

func (m *MinioService) GetMany(objectIDs []string) ([]string, error) {
	return m.client.GetMany(objectIDs)
}

func (m *MinioService) DeleteOne(objectID string) error {
	return m.client.DeleteOne(objectID)
}

func (m *MinioService) DeleteMany(objectIDs []string) error {
	return m.client.DeleteMany(objectIDs)
}
