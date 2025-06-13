package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"pastebin/backup/internal/infrastructure/storage"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client представляет клиент для работы с Redis
type Client struct {
	client *redis.Client
}

// NewClient создает новый клиент Redis
func NewClient(addr, password string, db int) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Проверяем подключение
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return &Client{
		client: client,
	}, nil
}

// Set сохраняет значение в Redis
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	err = c.client.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to set value: %v", err)
	}

	return nil
}

// Get получает значение из Redis
func (c *Client) Get(ctx context.Context, key string, value interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return storage.ErrNotFound
		}
		return fmt.Errorf("failed to get value: %v", err)
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal value: %v", err)
	}

	return nil
}

// Delete удаляет значение из Redis
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete value: %v", err)
	}

	return nil
}

// Exists проверяет существование ключа
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %v", err)
	}

	return exists > 0, nil
}

// Publish публикует сообщение в канал
func (c *Client) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	err = c.client.Publish(ctx, channel, data).Err()
	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	return nil
}

// Subscribe подписывается на канал
func (c *Client) Subscribe(ctx context.Context, channel string) (<-chan []byte, error) {
	pubsub := c.client.Subscribe(ctx, channel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %v", err)
	}

	ch := pubsub.Channel()
	messages := make(chan []byte)

	go func() {
		defer close(messages)
		defer pubsub.Close()

		for msg := range ch {
			messages <- []byte(msg.Payload)
		}
	}()

	return messages, nil
}
