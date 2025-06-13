package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/notification"
)

// Client представляет клиент для работы с MinIO
type Client struct {
	client     *minio.Client
	bucketName string
}

// NewClient создает новый клиент MinIO
func NewClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %v", err)
	}

	// Проверяем существование бакета
	exists, err := client.BucketExists(context.Background(), bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %v", err)
	}

	if !exists {
		err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	return &Client{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// Save сохраняет данные в MinIO
func (c *Client) Save(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	_, err := c.client.PutObject(ctx, c.bucketName, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{
			UserMetadata: metadata,
		})
	if err != nil {
		return fmt.Errorf("failed to save object: %v", err)
	}
	return nil
}

// Get получает данные из MinIO
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := c.client.GetObject(ctx, c.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %v", err)
	}

	return data, nil
}

// Delete удаляет данные из MinIO
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.client.RemoveObject(ctx, c.bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %v", err)
	}
	return nil
}

// List получает список объектов из MinIO
func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	var objects []string

	objectCh := c.client.ListObjects(ctx, c.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %v", object.Err)
		}
		objects = append(objects, object.Key)
	}

	return objects, nil
}

// ConfigureNotifications настраивает уведомления для бакета
func (c *Client) ConfigureNotifications(ctx context.Context, queueARN string) error {
	config := notification.Configuration{}
	config.AddQueue(notification.QueueConfig{
		Queue: queueARN,
		Events: []notification.EventType{
			notification.ObjectCreatedAll,
			notification.ObjectRemovedAll,
		},
	})

	err := c.client.SetBucketNotification(ctx, c.bucketName, config)
	if err != nil {
		return fmt.Errorf("failed to configure notifications: %v", err)
	}

	return nil
}
