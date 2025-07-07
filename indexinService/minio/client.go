package s3

import (
	"context"
	"fmt"
	"indexing_service/utils/workerpool"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Host     string
	Bucket   string
	RootUser string
	Password string
	SSL      bool
}

type Minio interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

type MinioClient struct {
	client *minio.Client
	pool   *workerpool.WorkerPool
	bucket string
}

func NewMinioClient(ctx context.Context, cfg Config, workers int) (*MinioClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := minio.New(
		cfg.Host,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				cfg.RootUser,
				cfg.Password,
				"",
			),
			Secure: cfg.SSL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	pool := workerpool.NewWorkerPool(workers)
	pool.Start()
	return &MinioClient{client: client, pool: pool, bucket: cfg.Bucket}, nil
}

func (m *MinioClient) Get(ctx context.Context, key string) ([]byte, error) {
	file, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error while getting URL for object %s: %w", key, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error while reading file: %w", err)
	}
	return data, nil
}

func (m *MinioClient) Close(ctx context.Context) {
	if m.pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		log.Printf("stopping worker pool...")
		m.pool.Stop()
		log.Printf("worker pool stopped successfully")
		close(done)
	}()

	select {
	case <-done:
		log.Printf("worker pool cleanup completed")
	case <-ctx.Done():
		fmt.Println("warning: worker pool stop timeout")
	}
}

func (m *MinioClient) Client() *minio.Client {
	return m.client
}

func (m *MinioClient) Pool() *workerpool.WorkerPool {
	return m.pool
}
