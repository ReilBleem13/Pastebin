package database

import (
	"pastebin/internal/models"
	"pastebin/pkg/dto"

	"github.com/jmoiron/sqlx"
)

type Authorization interface {
	CreateUser(user *dto.RequestNewUser) error
	GetHashPassword(email string) (string, error)
	GetUserIDByEmail(email string) (int, error)
}

type Minio interface {
	CreatePasta(pasta *models.Paste) error
	GetLink(hash string) (string, error)
	GetAll(pasta *models.Paste) error
	GetVisibility(hash string) (string, error)
	GetPastaByUserID(hash string) error
	AddViews(hash string) error
	GetHashPassword(hash string) (string, error)

	CheckPermission(userID int, hash string) (string, error)
}

type Repository struct {
	Authorization
	Minio
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		Authorization: NewAuthPostgres(db),
		Minio:         NewMinioPostgres(db),
	}
}
