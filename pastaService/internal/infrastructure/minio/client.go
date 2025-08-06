package minio

import (
	"context"
	"fmt"
	"net/http"
	"pastebin/internal/config"
	"pastebin/pkg/workerpool"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/theartofdevel/logging"
)

type minioClient struct {
	client *minio.Client
	pool   *workerpool.WorkerPool
}

func NewMinioClient(ctx context.Context, cfg config.MinioConfig, workers int) (*minioClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := minio.New(cfg.Addr, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.User, cfg.Password, ""),
		Secure: cfg.Ssl,
		Transport: &http.Transport{
			MaxIdleConns:    cfg.MaxIdleConns,
			IdleConnTimeout: cfg.IdleConnTimeout,
		},
		MaxRetries: cfg.MaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}
	logging.StringAttr("bucket", cfg.Bucket)
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

	return &minioClient{client: client, pool: pool}, nil
}

func (m *minioClient) Close(ctx context.Context) {
	if m.pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		logging.L(ctx).Debug("Stopping worker pool")
		m.pool.Stop()
		logging.L(ctx).Debug("Worker pool stopped successfully")
		close(done)
	}()

	select {
	case <-done:
		logging.L(ctx).Debug("worker pool cleanup completed")
	case <-ctx.Done():
		logging.L(ctx).Warn("warning: worker pool stop timeout")
	}
}

func (m *minioClient) Client() *minio.Client {
	return m.client
}

func (m *minioClient) Pool() *workerpool.WorkerPool {
	return m.pool
}
