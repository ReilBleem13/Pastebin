package repository

import (
	"context"
	"pastebin/internal/models"
)

type FileRepository interface {
	InitMinio() error

	StoreFile(ctx context.Context, owner string, data []byte, isPassword map[string]string) (models.Paste, error)
	GetFile(ctx context.Context, objectID string) (string, error)
	GetFiles(ctx context.Context, objectIDs []string) ([]string, error)
	DeleteFile(ctx context.Context, objectID string) error
	DeleteFiles(ctx context.Context, objectIDs []string) error

	PaginateFiles(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)
	PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)

	Close()
}
