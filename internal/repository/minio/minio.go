package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"pastebin/internal/models"
	"pastebin/pkg/helpers"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

func prtStr(s string) *string {
	return &s
}

func (m *minioClient) CreateOne(owner string, data []byte, isPassword map[string]string) (models.Paste, error) {
	objectID := owner + uuid.New().String() + ".txt"
	content := bytes.NewReader(data)

	_, err := m.mc.PutObject(
		context.Background(),
		m.cfg.BucketName,
		objectID,
		content,
		int64(len(data)),
		minio.PutObjectOptions{
			UserMetadata: isPassword,
		},
	)

	if err != nil {
		return models.Paste{}, fmt.Errorf("error occured while creating object: %v", err)
	}

	paste := models.Paste{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour * 24),
		Key:       objectID,
		Size:      int(len(data)),
	}

	return paste, nil
}

func (m *minioClient) CreateMany(data map[string]helpers.FileDataType) ([]string, error) {
	urls := make([]string, 0, len(data))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urlCh := make(chan string, len(data))

	var wg sync.WaitGroup

	for objectID, file := range data {
		wg.Add(1)
		go func(objectID string, file helpers.FileDataType) {
			defer wg.Done()
			_, err := m.mc.PutObject(ctx,
				m.cfg.BucketName,
				objectID,
				bytes.NewReader(file.Data),
				int64(len(file.Data)),
				minio.PutObjectOptions{},
			)
			if err != nil {
				cancel()
				return
			}

			url, err := m.mc.PresignedGetObject(ctx,
				m.cfg.BucketName,
				objectID,
				time.Second*24*60*60,
				nil,
			)
			if err != nil {
				cancel()
				return
			}
			urlCh <- url.String()
		}(objectID, file)
	}
	go func() {
		wg.Wait()
		close(urlCh)
	}()

	for url := range urlCh {
		urls = append(urls, url)
	}

	return urls, nil
}

func (m *minioClient) GetOne(objectID string) (string, error) {
	file, err := m.mc.GetObject(context.Background(), m.cfg.BucketName, objectID, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("error occured while getting URL for object %s: %v", objectID, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("error occured while reading file: %v", err)
	}

	return string(data), nil
}

func (m *minioClient) GetMany(objectIDs []string) ([]string, error) {
	urlCh := make(chan string, len(objectIDs))
	errCh := make(chan helpers.OperationError, len(objectIDs))

	var wg sync.WaitGroup
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, objectID := range objectIDs {
		wg.Add(1)
		go func(objectID string) {
			defer wg.Done()
			url, err := m.GetOne(objectID)
			if err != nil {
				errCh <- helpers.OperationError{
					ObjectID: objectID,
					Error:    err,
				}
				cancel()
				return
			}
			urlCh <- url
		}(objectID)
	}

	go func() {
		wg.Wait()
		close(urlCh)
		close(errCh)
	}()

	var urls []string
	var errs []error
	for url := range urlCh {
		urls = append(urls, url)
	}

	for err := range errCh {
		errs = append(errs, err.Error)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("error occured while getting objects: %v", errs)
	}
	return urls, nil
}

func (m *minioClient) DeleteOne(objectID string) error {
	err := m.mc.RemoveObject(context.Background(),
		m.cfg.BucketName,
		objectID,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return err
	}
	return nil
}

func (m *minioClient) DeleteMany(objectIDs []string) error {
	errCh := make(chan helpers.OperationError, len(objectIDs))

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, object := range objectIDs {
		go func(object string) {
			err := m.mc.RemoveObject(ctx,
				m.cfg.BucketName,
				object,
				minio.RemoveObjectOptions{},
			)
			if err != nil {
				errCh <- helpers.OperationError{
					ObjectID: object,
					Error:    err,
				}
				cancel()
			}
		}(object)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs []error
	for err := range errCh {
		errs = append(errs, err.Error)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to delete objects: %v", errs)
	}
	return nil
}

type Paste struct {
	Key          string
	LastModified string
}

func (m *minioClient) Paginate(maxKeys int, startAfter, prefix string) ([]string, string, error) {
	opts := minio.ListObjectsOptions{
		Recursive:  true,
		Prefix:     prefix,
		MaxKeys:    maxKeys,
		StartAfter: startAfter,
	}

	keys := []string{}
	objectCh := m.mc.ListObjects(context.Background(), m.cfg.BucketName, opts)

	for i := 0; i < maxKeys; i++ {
		object, ok := <-objectCh
		if !ok {
			break
		}
		if object.Err != nil {
			return []string{}, "", fmt.Errorf("failed to list objects: %v", object.Err)
		}

		objInfo, err := m.mc.StatObject(context.Background(), m.cfg.BucketName, object.Key, minio.StatObjectOptions{})
		if err != nil {
			return []string{}, "", err
		}

		hashPassword, exists := objInfo.UserMetadata["Has_password"]
		if exists && hashPassword == "true" {
			log.Printf("Skipping paste %s with password", object.Key)
			continue
		}

		keys = append(keys, object.Key)
	}

	nextKey := ""
	if len(keys) == maxKeys {
		nextKey = keys[len(keys)-1]
	}

	pastas, err := m.GetMany(keys)
	if err != nil {
		return []string{}, "", err
	}

	return pastas, nextKey, nil
}

func (m *minioClient) PaginateByUserID(maxKeys int, startAfter, prefix string) ([]string, string, error) {
	optsPublic := minio.ListObjectsOptions{
		Recursive:  true,
		Prefix:     "public:" + prefix,
		MaxKeys:    maxKeys,
		StartAfter: startAfter,
	}

	optsPrivate := minio.ListObjectsOptions{
		Recursive:  true,
		Prefix:     "private:" + prefix,
		MaxKeys:    maxKeys,
		StartAfter: startAfter,
	}

	type keyInfo struct {
		key  string
		time time.Time
	}

	keyCh := make(chan keyInfo, maxKeys*2)
	errCh := make(chan error, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		objectCh := m.mc.ListObjects(context.Background(), m.cfg.BucketName, optsPublic)
		for object := range objectCh {
			if object.Err != nil {
				errCh <- object.Err
				return
			}
			keyCh <- keyInfo{key: object.Key, time: object.LastModified}
		}
	}()

	go func() {
		defer wg.Done()
		objectCh := m.mc.ListObjects(context.Background(), m.cfg.BucketName, optsPrivate)
		for object := range objectCh {
			if object.Err != nil {
				errCh <- object.Err
				return
			}
			keyCh <- keyInfo{key: object.Key, time: object.LastModified}
		}
	}()

	go func() {
		wg.Wait()
		close(keyCh)
		close(errCh)
	}()

	collectedKeys := []keyInfo{}
collectLoop:
	for {
		select {
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, "", err
			}
		case ki, ok := <-keyCh:
			if !ok {
				break collectLoop
			}
			collectedKeys = append(collectedKeys, ki)
			if len(collectedKeys) >= maxKeys {
				break collectLoop
			}
		}
	}

	sort.Slice(collectedKeys, func(i, j int) bool {
		return collectedKeys[i].time.After(collectedKeys[j].time)
	})

	keys := make([]string, len(collectedKeys))
	for i, k := range collectedKeys {
		keys[i] = k.key
	}

	nextKey := ""
	if len(keys) == maxKeys {
		nextKey = keys[len(keys)-1]
	}

	pastas, err := m.GetMany(keys)
	if err != nil {
		return []string{}, "", err
	}
	return pastas, nextKey, nil
}
