package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func (m *S3) Store(ctx context.Context, owner string, data []byte, isPassword map[string]string) (*models.Pasta, error) {
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
		CreatedAt: time.Now(),
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

// сделать аргумент, чтобы доставалось с паролей / без пароля / без разницы
/*
 obj, err := client.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
  if err != nil {
      // Ошибка соединения или некорректные параметры, но не "объект не найден"
  }
  // Ошибка "объект не найден" появится только при чтении:
  buf := make([]byte, 10)
  _, err = obj.Read(buf)
*/
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

// func (m *S3) PaginateFiles(ctx context.Context, limit int, startAfter, prefix string) (*[]string, string, error) {
// 	if ctx.Err() != nil {
// 		return nil, "", ctx.Err()
// 	}

// 	opts := minio.ListObjectsOptions{
// 		Recursive: true,
// 		Prefix:    prefix,
// 		MaxKeys:   limit,
// 	}

// 	log.Printf("StartAfter: %s", startAfter)

// 	if startAfter != "" {
// 		opts.StartAfter = startAfter
// 	}

// 	keys := []string{}
// 	objectCh := m.client.ListObjects(ctx, m.bucket, opts)

// 	for i := 0; i < limit; i++ {
// 		object, ok := <-objectCh
// 		if !ok {
// 			break
// 		}
// 		if object.Err != nil {
// 			return nil, "", fmt.Errorf("failed to list objects: %v", object.Err)
// 		}

// 		fmt.Printf("#%d: %s\n", i, object.Key)

// 		objInfo, err := m.client.StatObject(ctx, m.bucket, object.Key, minio.StatObjectOptions{})
// 		if err != nil {
// 			return nil, "", err
// 		}

// 		hashPassword, exists := objInfo.UserMetadata["Has_password"]
// 		if exists && hashPassword == "true" {
// 			continue
// 		}

// 		keys = append(keys, object.Key)
// 	}

// 	nextKey := ""
// 	if len(keys) == limit {
// 		nextKey = keys[len(keys)-1]
// 	}

// 	_, err := m.GetFiles(ctx, keys)
// 	if err != nil {
// 		return nil, "", err
// 	}
// 	return nil, nextKey, nil
// }

// type keyInfo struct {
// 	key  string
// 	time time.Time
// }

// func (m *S3) PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) (*[]string, string, error) {
// 	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
// 	defer cancel()

// 	optsPublic := minio.ListObjectsOptions{
// 		Recursive:  true,
// 		Prefix:     publucPrefix + prefix,
// 		MaxKeys:    maxKeys,
// 		StartAfter: startAfter,
// 	}

// 	optsPrivate := minio.ListObjectsOptions{
// 		Recursive:  true,
// 		Prefix:     privatePrefix + prefix,
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

// 	_, err := m.GetFiles(ctx, keys)
// 	if err != nil {
// 		return nil, "", err
// 	}
// 	return nil, nextKey, nil
// }

// func (m *S3) PaginateV1(ctx context.Context) error {
// 	return nil
// }
