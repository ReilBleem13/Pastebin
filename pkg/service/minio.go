package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"pastebin/pkg/helpers"
	"pastebin/pkg/models"
	"pastebin/pkg/repository/database"
	"pastebin/pkg/repository/minio"
	"pastebin/pkg/repository/redis"
	"strings"
)

type MinioService struct {
	client minio.Client
	redis  redis.Redis
	repo   database.Minio
}

func NewMinioService(minio minio.Client, redis redis.Redis, repo database.Minio) *MinioService {
	return &MinioService{
		client: minio,
		redis:  redis,
	}
}

func (m *MinioService) CreateOne(ctx context.Context, data []byte) (models.Paste, error) {
	pasta, err := m.client.CreateOne(data)
	if err != nil {
		return models.Paste{}, fmt.Errorf("unable ti save the file: %v", err)
	}
	hash := sha256.Sum256([]byte(pasta.StorageKey))
	hashStr := hex.EncodeToString(hash[:])
	pasta.Hash = hashStr

	if err := m.redis.Add(ctx, &pasta, data); err != nil {
		return models.Paste{}, fmt.Errorf("unable add to redis: %v", err)
	}

	return pasta, nil
}

func (m *MinioService) CreateMany(files map[string]helpers.FileDataType) ([]string, error) {
	return m.client.CreateMany(files)
}

func (m *MinioService) GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error {
	keyMeta := fmt.Sprintf("meta:%s", pasta.Hash)
	keyData := fmt.Sprintf("data:%s", pasta.Hash)

	if flag {
		err := m.redis.Get(ctx, pasta, keyData, keyMeta)
		if err != nil {
			if strings.Contains(err.Error(), "key doesn't exists") {
				if err := m.repo.GetAll(pasta); err != nil {
					return err
				}

				pasta.Text, err = m.client.GetOne(pasta.ObjectID)
				if err != nil {
					return err
				}
			}
			return err
		}
		return nil
	}
	if err := m.redis.Get(ctx, pasta, keyData, ""); err != nil {
		if strings.Contains(err.Error(), "key doesn't exists") {
			pasta.Text, err = m.client.GetOne(pasta.ObjectID)
			if err != nil {
				return err
			}
		}
		return err
	}

	return nil
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
