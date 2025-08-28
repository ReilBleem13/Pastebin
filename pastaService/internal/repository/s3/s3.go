package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	domain "pastebin/internal/domain/repository"
	"pastebin/internal/models"
	"pastebin/pkg/workerpool"
	"sync"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type S3 struct {
	client *minio.Client
	pool   *workerpool.WorkerPool
	bucket string
}

func NewS3(client *minio.Client, pool *workerpool.WorkerPool, bucket string) domain.S3 {
	return &S3{
		client: client,
		pool:   pool,
		bucket: bucket,
	}
}

const (
	privatePrefix string = "private:"
	publucPrefix  string = "public:"
	hasPassword   string = "Has_password"
	batchSize     int    = 500
)

func (m *S3) Store(ctx context.Context, owner string, data []byte) (*models.Pasta, error) {
	objectID := owner + uuid.New().String() + ".txt"
	content := bytes.NewReader(data)

	_, err := m.client.PutObject(
		ctx,
		m.bucket,
		objectID,
		content,
		int64(len(data)),
		minio.PutObjectOptions{},
	)

	if err != nil {
		return nil, fmt.Errorf("error while creating object: %w", err)
	}

	paste := &models.Pasta{
		ObjectID: objectID,
		Size:     int(len(data)),
	}
	return paste, nil
}

// /*
// nil - без разницы есть пароль или нет
// true - пароль должен быть
// false - пароля не должно быть
// */
// func isPasswordNeed(isPassword *bool, objInfo *minio.ObjectInfo) bool {
// 	if isPassword == nil {
// 		return true
// 	} else if *isPassword {
// 		hashPassword, exists := objInfo.UserMetadata[hasPassword]
// 		if exists && hashPassword == "true" {
// 			return true
// 		} else {
// 			return false
// 		}
// 	} else {
// 		hashPassword, exists := objInfo.UserMetadata[hasPassword]
// 		if exists && hashPassword == "false" {
// 			return true
// 		} else {
// 			return false
// 		}
// 	}
// }

func (m *S3) Get(ctx context.Context, key string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	file, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("error while getting URL for object %s: %w", key, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("error while reading file: %w", err)
	}
	return string(data), nil
}

func (m *S3) Update(ctx context.Context, newText *bytes.Reader, objectID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	uploadInfo, err := m.client.PutObject(ctx, m.bucket, objectID, newText, int64(newText.Size()), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to put new object: %w", err)
	}

	log.Printf("uploadInfo: %v", uploadInfo)
	return nil
}

func (m *S3) Delete(ctx context.Context, key string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	err := m.client.RemoveObject(ctx,
		m.bucket,
		key,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("error while deleting file: %w", err)
	}
	return nil
}

func (m *S3) DeleteMany(ctx context.Context, keys []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var allErrs []error

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		objectCh := make(chan minio.ObjectInfo)

		go func() {
			defer close(objectCh)
			for _, key := range batch {
				select {
				case objectCh <- minio.ObjectInfo{Key: key}:
				case <-ctx.Done():
					return
				}
			}
		}()

		for rErr := range m.client.RemoveObjects(ctx, m.bucket, objectCh, minio.RemoveObjectsOptions{}) {
			if rErr.Err != nil {
				allErrs = append(allErrs, fmt.Errorf("delete %s: %w", rErr.ObjectName, rErr.Err))
			}
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	if len(allErrs) > 0 {
		return errors.Join(allErrs...)
	}
	return nil
}

func (m *S3) GetFiles(ctx context.Context, keys []string) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		index int
		text  string
	}

	textCh := make(chan result, len(keys))
	errCh := make(chan error, len(keys))

	wg := &sync.WaitGroup{}
	for i, key := range keys {
		wg.Add(1)
		index := i
		k := key

		m.pool.Tasks <- func() {
			defer wg.Done()

			if ctx.Err() != nil {
				return
			}

			text, err := m.Get(ctx, k)
			if err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
				return
			}

			if text == "" {
				log.Printf("empty text for key %s", k)
				return
			}

			select {
			case textCh <- result{index: index, text: text}:
			case <-ctx.Done():
				return
			}
		}
	}

	go func() {
		wg.Wait()
		close(textCh)
		close(errCh)
	}()

	texts := make([]string, len(keys))
	for text := range textCh {
		texts[text.index] = text.text
	}

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("error while getting text from s3: %v", errs)
	}
	return texts, nil
}
