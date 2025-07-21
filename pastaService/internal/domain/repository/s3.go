package domain

import (
	"bytes"
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
	"time"
)

//go:generate mockgen -source=s3.go -destination=../mocks/repository/s3.go -package=mocks

type S3 interface {
	Store(ctx context.Context, owner string, data []byte, isPassword map[string]string, timeNow time.Time) (*models.Pasta, error)
	Get(ctx context.Context, key string, password *bool) (*string, *time.Time, error)
	Delete(ctx context.Context, hash string) error
	Update(ctx context.Context, newText *bytes.Reader, objectID string) error

	GetFiles(ctx context.Context, objectIDs []string, password *bool) (*[]dto.Entry, error)
}
