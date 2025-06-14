package minio

import (
	"context"
	"fmt"
	"log"
	"pastebin/internal/models"
	"pastebin/pkg/workerpool"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	MinioEndPoint     string
	BucketName        string
	MinioRootUser     string
	MinioRootPassword string
	MinioUseSSL       bool
}

type FileRepository interface {
	InitMinio() error

	StoreFile(ctx context.Context, owner string, data []byte, isPassword map[string]string) (models.Paste, error)
	GetFile(ctx context.Context, objectID string) (string, error)
	GetFiles(ctx context.Context, objectIDs []string) ([]string, error)
	DeleteFile(ctx context.Context, objectID string) error
	DeleteFiles(ctx context.Context, objectIDs []string) error

	PaginateFiles(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)
	PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)

	Close()
}

type minioClient struct {
	mc   *minio.Client
	cfg  Config
	pool *workerpool.WorkerPool
}

func NewMinioClient(cfg Config, workers int) FileRepository {
	pool := workerpool.NewWorkerPool(workers)
	pool.Start()

	return &minioClient{
		cfg:  cfg,
		pool: pool,
	}
}

func (m *minioClient) InitMinio() error {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := minio.New(
		m.cfg.MinioEndPoint,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				m.cfg.MinioRootUser,
				m.cfg.MinioRootPassword,
				"",
			),
			Secure: m.cfg.MinioUseSSL, // TODO: change if error
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create minio client: %w", err)
	}

	m.mc = client
	exists, err := m.mc.BucketExists(ctx, m.cfg.BucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err := m.mc.MakeBucket(ctx, m.cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}

func (m *minioClient) Close() {
	if m.pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		log.Printf("stopping worker pool...")
		m.pool.Stop()
		log.Printf("worker pool stopped successfully")
		close(done)
	}()

	select {
	case <-done: // успешно остановился
		log.Printf("worker pool cleanup completed")
	case <-ctx.Done(): // контекст завершен
		fmt.Println("warning: worker pool stop timeout")
	}
}
