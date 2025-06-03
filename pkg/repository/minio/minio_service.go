package minio

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"pastebin/pkg/helpers"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

func (m *minioClient) CreateOne(file helpers.FileDataType) (string, error) {
	objectID := uuid.New().String()
	log.Println(objectID)
	reader := bytes.NewReader(file.Data)

	_, err := m.mc.PutObject(
		context.Background(),
		m.cfg.BucketName,
		objectID,
		reader,
		int64(len(file.Data)),
		minio.PutObjectOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("error occured while creating object %s: %v", file.FileName, err)
	}

	url, err := m.mc.PresignedGetObject(context.Background(), m.cfg.BucketName, objectID, time.Second*24*60*60, nil)
	if err != nil {
		return "", fmt.Errorf("error occured while creating URL for object %s: %v", file.FileName, err)
	}
	return url.String(), nil
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
	url, err := m.mc.PresignedGetObject(
		context.Background(),
		m.cfg.BucketName,
		objectID,
		time.Second*24*60*60,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("error occured while getting URL for object %s: %v", objectID, err)
	}
	return url.String(), nil
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
