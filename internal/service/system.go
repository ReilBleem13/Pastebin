package service

import (
	"context"
	"log"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/minio"
	"time"
)

type SystemSerivce struct {
	client minio.FileRepository
	repo   database.System
}

func NewSystemSerivce(minio minio.FileRepository, repo database.System) *SystemSerivce {
	return &SystemSerivce{
		client: minio,
		repo:   repo,
	}
}

func (m *SystemSerivce) BackGroundDel() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			for {
				deletedCount, err := m.repo.DeleteExriredMetadata(ctx)
				if err != nil {
					log.Printf("cleanup error: %v", err)
				}
				if deletedCount == 0 {
					break
				}

				log.Printf("Количество удалённых строк: %d", deletedCount)
				time.Sleep(100 * time.Millisecond)
			}
			cancel()
		}
	}()
}
