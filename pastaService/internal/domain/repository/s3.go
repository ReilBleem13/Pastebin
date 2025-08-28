package domain

import (
	"bytes"
	"context"
	"pastebin/internal/models"
)

//go:generate mockgen -source=s3.go -destination=../mocks/repository/s3.go -package=mocks

type S3 interface {
	Store(ctx context.Context, owner string, data []byte) (*models.Pasta, error)
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, hash string) error
	Update(ctx context.Context, newText *bytes.Reader, objectID string) error

	GetFiles(ctx context.Context, keys []string) ([]string, error)
	DeleteMany(ctx context.Context, keys []string) error
}
