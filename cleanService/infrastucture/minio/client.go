package s3

import (
	"cleanService/config"
	"cleanService/utils/workerpool"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const batchSize = 500

type Minio interface {
	Delete(ctx context.Context, keys []string) error
}

type MinioClient struct {
	client *minio.Client
	pool   *workerpool.WorkerPool
	bucket string
}

func NewMinioClient(ctx context.Context, cfg config.MinioConfig, workers int) (*MinioClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := minio.New(
		cfg.Host,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				cfg.Rootuser,
				cfg.Password,
				"",
			),
			Secure: cfg.Ssl,
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

func (m *MinioClient) Delete(ctx context.Context, keys []string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var errs []error
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		objectCh := make(chan minio.ObjectInfo)
		go func(batch []string) {
			defer close(objectCh)
			for _, key := range batch {
				objectCh <- minio.ObjectInfo{Key: key}
			}
		}(batch)

		for rErr := range m.client.RemoveObjects(ctx, m.bucket, objectCh, minio.RemoveObjectsOptions{}) {
			if rErr.Err != nil {
				errs = append(errs, fmt.Errorf("error deleting %s: %w", rErr.ObjectName, rErr.Err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during deletion: %v", errs)
	}
	return nil
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
