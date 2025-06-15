package service

import (
	"context"
	"log"
	"pastebin/internal/domain/repository"
)

type ExpiredDeleteService struct {
	repo  repository.MinioRepository
	minio repository.FileRepository
}

func NewExpiredDeleteService(repo repository.MinioRepository, minio repository.FileRepository) *ExpiredDeleteService {
	return &ExpiredDeleteService{repo: repo, minio: minio}
}

func (s *ExpiredDeleteService) CleanupExpiredPasta(ctx context.Context) error {
	expiredPasta, err := s.repo.GetKeysExpiredPasta(ctx)
	if err != nil {
		return err
	}

	log.Println("Удаление в minio")
	if err := s.minio.DeleteFiles(ctx, expiredPasta); err != nil {
		log.Printf("Failed to delete expired pasta from MinIO: %v", err)
	}

	log.Println("Удаление в db")
	if err := s.repo.DeleteExpiredPasta(ctx, expiredPasta); err != nil {
		log.Printf("Failed to delete expired pasta from DB: %v", err)
	}

	log.Printf("Удаление завершено. Удалено %d паст", len(expiredPasta))
	return nil
}
