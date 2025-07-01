package domain

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"
	"time"
)

type S3 interface {
	Store(ctx context.Context, owner string, data []byte, isPassword map[string]string) (*models.Pasta, error)
	Get(ctx context.Context, key string) (*string, *time.Time, error)
	Delete(ctx context.Context, hash string) error

	GetFiles(context.Context, []string) (*[]dto.Entry, error)
	// DeleteFile(ctx context.Context, objectID string) error
	// DeleteFiles(ctx context.Context, objectIDs []string) error

	PaginateFiles(ctx context.Context, limit int, startAfter, prefix string) (*[]string, string, error)
	PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) (*[]string, string, error)
}
