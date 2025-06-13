package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound возвращается, когда запрашиваемый объект не найден
var ErrNotFound = errors.New("object not found")

// Storage определяет интерфейс для работы с хранилищем
type Storage interface {
	// Save сохраняет данные в хранилище
	Save(ctx context.Context, key string, data io.Reader, metadata map[string]string) error
	// Get получает данные из хранилища
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete удаляет данные из хранилища
	Delete(ctx context.Context, key string) error
	// List возвращает список объектов в хранилище
	List(ctx context.Context, prefix string) ([]string, error)
}

// Cache определяет интерфейс для работы с кэшем
type Cache interface {
	// Set сохраняет значение в кэше
	Set(ctx context.Context, key string, value interface{}, ttl int) error
	// Get получает значение из кэша
	Get(ctx context.Context, key string, value interface{}) error
	// Delete удаляет значение из кэша
	Delete(ctx context.Context, key string) error
	// Exists проверяет существование ключа
	Exists(ctx context.Context, key string) (bool, error)
}

// Database определяет интерфейс для работы с базой данных
type Database interface {
	// Save сохраняет запись в базу данных
	Save(ctx context.Context, collection string, document interface{}) error
	// Get получает запись из базы данных
	Get(ctx context.Context, collection string, id string, document interface{}) error
	// Delete удаляет запись из базы данных
	Delete(ctx context.Context, collection string, id string) error
	// List возвращает список записей из базы данных
	List(ctx context.Context, collection string, filter interface{}, documents interface{}) error
}
