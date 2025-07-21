package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	domain "pastebin/internal/domain/repository"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
	"pastebin/pkg/workerpool"
	"sort"
	"sync"
	"time"

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
)

func (m *S3) Store(ctx context.Context, owner string, data []byte, isPassword map[string]string, timeNow time.Time) (*models.Pasta, error) {
	objectID := owner + uuid.New().String() + ".txt"
	content := bytes.NewReader(data)

	_, err := m.client.PutObject(
		ctx,
		m.bucket,
		objectID,
		content,
		int64(len(data)),
		minio.PutObjectOptions{
			UserMetadata: isPassword,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error while creating object: %w", err)
	}

	paste := &models.Pasta{
		CreatedAt: timeNow,
		ObjectID:  objectID,
		Size:      int(len(data)),
	}
	return paste, nil
}

/*
nil - без разницы есть пароль или нет
true - пароль должен быть
false - пароля не должно быть
*/
func isPasswordNeed(isPassword *bool, objInfo *minio.ObjectInfo) bool {
	if isPassword == nil {
		return true
	} else if *isPassword {
		hashPassword, exists := objInfo.UserMetadata[hasPassword]
		if exists && hashPassword == "true" {
			return true
		} else {
			return false
		}
	} else {
		hashPassword, exists := objInfo.UserMetadata[hasPassword]
		if exists && hashPassword == "false" {
			return true
		} else {
			return false
		}
	}
}

func (m *S3) Get(ctx context.Context, key string, password *bool) (*string, *time.Time, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	file, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting URL for object %s: %w", key, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("error while reading file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get stat of object: %w", err)
	}
	result := string(data)

	if isPasswordNeed(password, &stat) {
		return &result, &stat.LastModified, nil
	}
	return nil, nil, nil
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
func (m *S3) GetFiles(ctx context.Context, keys []string, password *bool) (*[]dto.Entry, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dataCh := make(chan dto.Entry, len(keys))
	errCh := make(chan error, len(keys))

	handleError := func(err error) {
		select {
		case errCh <- err:
		default:
			cancel()
			return
		}
	}

	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		key := key

		m.pool.Tasks <- func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					handleError(fmt.Errorf("panic in GetFile: %v", r))
				}
			}()

			if ctx.Err() != nil {
				return
			}

			data, lastModified, err := m.Get(ctx, key, password)
			if err != nil {
				handleError(err)
				return
			}
			if data == nil || lastModified == nil {
				// handleError(fmt.Errorf("text or lastmodified is empty"))
				return
			}

			select {
			case dataCh <- dto.Entry{Text: *data, ObjectID: key, Time: *lastModified}:
			case <-ctx.Done():
				return
			}
		}
	}

	go func() {
		wg.Wait()
		close(dataCh)
		close(errCh)
	}()

	var datas []dto.Entry
	var errs []error

	for data := range dataCh {
		datas = append(datas, data)
	}

	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("error while getting objects: %v", errs)
	}

	sort.Slice(datas, func(i, j int) bool {
		return datas[i].Time.Before(datas[j].Time)
	})
	return &datas, nil
}
