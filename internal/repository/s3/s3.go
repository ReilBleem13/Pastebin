package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"pastebin/internal/domain"
	"pastebin/internal/models"
	"pastebin/pkg/workerpool"
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

func (m *S3) StoreFile(ctx context.Context, owner string, data *[]byte, isPassword map[string]string) (*models.Pasta, error) {
	objectID := owner + uuid.New().String() + ".txt"
	content := bytes.NewReader(*data)

	_, err := m.client.PutObject(
		ctx,
		m.bucket,
		objectID,
		content,
		int64(len(*data)),
		minio.PutObjectOptions{
			UserMetadata: isPassword,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error while creating object: %v", err)
	}

	paste := &models.Pasta{
		CreatedAt: time.Now(),
		Key:       objectID,
		Size:      int(len(*data)),
	}
	return paste, nil
}

func (m *S3) GetFile(ctx context.Context, key string) (*string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	file, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error while getting URL for object %s: %v", key, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error while reading file: %v", err)
	}

	result := string(data)
	return &result, nil
}

// func (m *S3) GetFiles(ctx context.Context, keys []string) ([]string, error) {
// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()

// 	dataCh := make(chan string, len(keys))
// 	errCh := make(chan error, len(keys))

// 	handleError := func(err error) {
// 		select {
// 		case errCh <- err:
// 		default:
// 			cancel()
// 			return
// 		}
// 	}

// 	var wg sync.WaitGroup
// 	for _, key := range keys {
// 		wg.Add(1)
// 		key := key

// 		m.pool.Tasks <- func() {
// 			defer wg.Done()
// 			defer func() {
// 				if r := recover(); r != nil {
// 					handleError(fmt.Errorf("panic in GetFile: %v", r))
// 				}
// 			}()

// 			if ctx.Err() != nil {
// 				return
// 			}

// 			data, err := m.GetFile(ctx, key)
// 			if err != nil {
// 				handleError(err)
// 				return
// 			}

// 			select {
// 			case dataCh <- data:
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}

// 	go func() {
// 		wg.Wait()
// 		close(dataCh)
// 		close(errCh)
// 	}()

// 	var datas []string
// 	var errs []error

// 	for data := range dataCh {
// 		datas = append(datas, data)
// 	}

// 	for err := range errCh {
// 		errs = append(errs, err)
// 	}

// 	if len(errs) > 0 {
// 		return nil, fmt.Errorf("error while getting objects: %v", errs)
// 	}
// 	return datas, nil
// }

// func (m *S3) DeleteFile(ctx context.Context, key string) error {
// 	if ctx.Err() != nil {
// 		return ctx.Err()
// 	}

// 	err := m.client.RemoveObject(ctx,
// 		m.bucket,
// 		key,
// 		minio.RemoveObjectOptions{},
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (m *S3) DeleteFiles(ctx context.Context, keys []string) error {
// 	if len(keys) == 0 {
// 		return nil
// 	}

// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()

// 	errCh := make(chan error, len(keys))
// 	var wg sync.WaitGroup

// 	handleError := func(err error) {
// 		select {
// 		case errCh <- err:
// 		default:
// 			cancel()
// 			return
// 		}
// 	}

// 	for _, key := range keys {
// 		wg.Add(1)
// 		key := key
// 		m.pool.Tasks <- func() {
// 			defer wg.Done()
// 			defer func() {
// 				if r := recover(); r != nil {
// 					handleError(fmt.Errorf("panic in DeleteFile: %v", r))
// 				}
// 			}()

// 			if ctx.Err() != nil {
// 				return
// 			}

// 			err := m.DeleteFile(ctx, key)
// 			if err != nil {
// 				handleError(err)
// 				return
// 			}
// 		}
// 	}

// 	go func() {
// 		wg.Wait()
// 		close(errCh)
// 	}()

// 	var errs []error
// 	for err := range errCh {
// 		errs = append(errs, err)
// 	}

// 	if len(errs) > 0 {
// 		return fmt.Errorf("failed to delete objects: %v", errs)
// 	}
// 	return nil
// }

// type Paste struct {
// 	Key          string
// 	LastModified string
// }

// func (m *S3) PaginateFiles(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error) {
// 	if ctx.Err() != nil {
// 		return []string{}, "", ctx.Err()
// 	}

// 	opts := minio.ListObjectsOptions{
// 		Recursive:  true,
// 		Prefix:     prefix,
// 		MaxKeys:    maxKeys,
// 		StartAfter: startAfter,
// 	}

// 	keys := []string{}
// 	objectCh := m.client.ListObjects(ctx, m.bucket, opts)

// 	for i := 0; i < maxKeys; i++ {
// 		object, ok := <-objectCh
// 		if !ok {
// 			break
// 		}
// 		if object.Err != nil {
// 			return []string{}, "", fmt.Errorf("failed to list objects: %v", object.Err)
// 		}

// 		objInfo, err := m.client.StatObject(ctx, m.bucket, object.Key, minio.StatObjectOptions{})
// 		if err != nil {
// 			return []string{}, "", err
// 		}

// 		hashPassword, exists := objInfo.UserMetadata["Has_password"]
// 		if exists && hashPassword == "true" {
// 			continue
// 		}

// 		keys = append(keys, object.Key)
// 	}

// 	nextKey := ""
// 	if len(keys) == maxKeys {
// 		nextKey = keys[len(keys)-1]
// 	}

// 	pastas, err := m.GetFiles(ctx, keys)
// 	if err != nil {
// 		return []string{}, "", err
// 	}

// 	return pastas, nextKey, nil
// }

// type keyInfo struct {
// 	key  string
// 	time time.Time
// }

// func (m *S3) PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error) {
// 	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
// 	defer cancel()

// 	optsPublic := minio.ListObjectsOptions{
// 		Recursive:  true,
// 		Prefix:     "public:" + prefix,
// 		MaxKeys:    maxKeys,
// 		StartAfter: startAfter,
// 	}

// 	optsPrivate := minio.ListObjectsOptions{
// 		Recursive:  true,
// 		Prefix:     "private:" + prefix,
// 		MaxKeys:    maxKeys,
// 		StartAfter: startAfter,
// 	}

// 	keyCh := make(chan keyInfo, maxKeys*2)
// 	errCh := make(chan error, 1)

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	handleError := func(err error) {
// 		select {
// 		case errCh <- err:
// 		default:
// 			cancel()
// 		}
// 	}

// 	go func() {
// 		defer wg.Done()
// 		defer func() {
// 			if r := recover(); r != nil {
// 				handleError(fmt.Errorf("panic in ListObjects: %v", r))
// 			}
// 		}()

// 		if ctx.Err() != nil {
// 			return
// 		}

// 		objectCh := m.client.ListObjects(ctx, m.bucket, optsPublic)
// 		for object := range objectCh {
// 			if object.Err != nil {
// 				handleError(object.Err)
// 				return
// 			}
// 			select {
// 			case keyCh <- keyInfo{key: object.Key, time: object.LastModified}:
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		defer func() {
// 			if r := recover(); r != nil {
// 				handleError(fmt.Errorf("panic in ListObjects: %v", r))
// 			}
// 		}()

// 		if ctx.Err() != nil {
// 			return
// 		}

// 		objectCh := m.client.ListObjects(ctx, m.bucket, optsPrivate)
// 		for object := range objectCh {
// 			if object.Err != nil {
// 				handleError(object.Err)
// 				return
// 			}
// 			select {
// 			case keyCh <- keyInfo{key: object.Key, time: object.LastModified}:
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()

// 	go func() {
// 		wg.Wait()
// 		close(keyCh)
// 		close(errCh)
// 	}()

// 	collectedKeys := []keyInfo{}
// collectLoop:
// 	for {
// 		select {
// 		case err, ok := <-errCh:
// 			if ok && err != nil {
// 				return nil, "", err
// 			}
// 		case ki, ok := <-keyCh:
// 			if !ok {
// 				break collectLoop
// 			}
// 			collectedKeys = append(collectedKeys, ki)
// 			if len(collectedKeys) >= maxKeys {
// 				break collectLoop
// 			}
// 		}
// 	}

// 	sort.Slice(collectedKeys, func(i, j int) bool {
// 		return collectedKeys[i].time.After(collectedKeys[j].time)
// 	})

// 	keys := make([]string, len(collectedKeys))
// 	for i, k := range collectedKeys {
// 		keys[i] = k.key
// 	}

// 	nextKey := ""
// 	if len(keys) == maxKeys {
// 		nextKey = keys[len(keys)-1]
// 	}

// 	pastas, err := m.GetFiles(ctx, keys)
// 	if err != nil {
// 		return nil, "", err
// 	}
// 	return pastas, nextKey, nil
// }
