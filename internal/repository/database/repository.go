package database

import (
	"context"
	"pastebin/internal/models"
	"pastebin/pkg/dto"

	"github.com/jmoiron/sqlx"
)

type Authorization interface {
	CreateUser(ctx context.Context, user *dto.RequestNewUser) error
	GetHashPassword(ctx context.Context, email string) (string, error)
	GetUserIDByEmail(ctx context.Context, email string) (int, error)
}

type MinioMetadata interface {
	CreateMetadata(ctx context.Context, pasta *models.Paste) error

	GetKey(ctx context.Context, hash string) (string, error)
	GetVisibility(ctx context.Context, hash string) (string, error)
	GetMetadata(ctx context.Context, pasta *models.Paste) error
	GetPassword(ctx context.Context, hash string) (string, error)
	GetKeys(ctx context.Context, userID int) ([]string, error)

	AddViews(ctx context.Context, hash string) error
	CheckPermission(ctx context.Context, userID int, hash string) (string, error)
	DeleteMetadata(ctx context.Context, hash string) (string, error)
}

type System interface {
	DeleteExriredMetadata(ctx context.Context) (int, error)
}

type Repository struct {
	Authorization
	MinioMetadata
	System
}

func NewRepository(db *sqlx.DB) *Repository {

	return &Repository{
		Authorization: NewAuthPostgres(db),
		MinioMetadata: NewMinioPostgres(db),
		System:        NewSystemPostgres(db),
	}
}
