package database

import (
	"pastebin/pkg/models"

	"github.com/jmoiron/sqlx"
)

type Authorization interface{}

type Minio interface {
	CreatePasta(pasta models.Paste) error
	GetLink(hash string) (string, error)
	GetAll(pasta *models.PasteWithData) error
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
