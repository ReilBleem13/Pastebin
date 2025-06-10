package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"pastebin/internal/models"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/minio"
	"pastebin/internal/repository/redis"
	"pastebin/pkg/helpers"
	"strings"
	"sync"
	"time"
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
		repo:   repo,
	}
}

func (m *MinioService) CreateOne(ctx context.Context, data []byte) (models.Paste, error) {
	pasta, err := m.client.CreateOne(data)
	if err != nil {
		return models.Paste{}, fmt.Errorf("unable ti save the file: %v", err)
	}
	hash := sha256.Sum256([]byte(pasta.Key))
	hashStr := hex.EncodeToString(hash[:])
	pasta.Hash = hashStr

	if err := m.redis.AddText(ctx, pasta.Hash, data); err != nil {
		return models.Paste{}, err
	}

	// if err := m.redis.AddMeta(ctx, &pasta); err != nil {
	// 	return models.Paste{}, err
	// }

	return pasta, nil
}

func (m *MinioService) CreateMany(files map[string]helpers.FileDataType) ([]string, error) {
	return m.client.CreateMany(files)
}

func (m *MinioService) GetText(ctx context.Context, pasta *models.PasteWithData, keyData string) error {
	start := time.Now()
	err := m.redis.GetText(ctx, pasta, keyData)
	if err != nil {
		if strings.Contains(err.Error(), "key doesn't exists") {
			pasta.Text, err = m.client.GetOne(pasta.Metadata.Key)
			if err != nil {
				return err
			}
			log.Println("текст из MINIO")

			if err := m.redis.AddText(ctx, pasta.Metadata.Hash, []byte(pasta.Text)); err != nil {
				return err
			}
			log.Println("текст добавлен в Redis")
		} else {
			return err
		}
	}
	log.Println("Обработка текста в сервисе. Время:", time.Since(start).Seconds())
	return nil
}

func (m *MinioService) GetOne(ctx context.Context, pasta *models.PasteWithData, flag bool) error {
	start := time.Now()

	key, err := m.repo.GetLink(pasta.Metadata.Hash)
	if err != nil {
		return err
	}
	pasta.Metadata.Key = key

	var wg sync.WaitGroup
	var textErr, metaErr error

	keyMeta := fmt.Sprintf("meta:%s", pasta.Metadata.Hash)
	keyData := fmt.Sprintf("data:%s", pasta.Metadata.Hash)

	if !flag {
		if err := m.GetText(ctx, pasta, keyData); err != nil {
			return err
		}

		if err := m.repo.AddViews(pasta.Metadata.Hash); err != nil {
			return err
		}

		return nil
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		textErr = m.GetText(ctx, pasta, keyData)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := m.redis.GetMeta(ctx, pasta, keyMeta)
		if err != nil {
			if strings.Contains(err.Error(), "key doesn't exists") {
				if err := m.repo.GetAll(&pasta.Metadata); err != nil {
					metaErr = err
					return
				}
				log.Println("Метаданные из DB")

				if err := m.redis.AddMeta(ctx, &pasta.Metadata); err != nil {
					metaErr = err
					return
				}
				log.Println("Метаданные добавлены в Redis")
			} else {
				metaErr = err
			}
		}
	}()
	wg.Wait()

	if textErr != nil {
		return textErr
	}
	if metaErr != nil {
		return metaErr
	}

	if err := m.repo.AddViews(pasta.Metadata.Hash); err != nil {
		return err
	}
	log.Println("Обработка в сервисе. Время:", time.Since(start).Seconds())
	return nil
}

func (m *MinioService) GetMany(objectIDs []string) ([]string, error) {
	return m.client.GetMany(objectIDs)
}

func (m *MinioService) DeleteOne(hash string) error {
	key, err := m.repo.DeleteMetadata(hash)
	if err != nil {
		return err
	}
	return m.client.DeleteOne(key)
}

func (m *MinioService) DeleteMany(objectIDs []string) error {
	return m.client.DeleteMany(objectIDs)
}
