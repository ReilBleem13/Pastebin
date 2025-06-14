package postgres

import (
	"pastebin/internal/domain/repository"

	"github.com/jmoiron/sqlx"
)

type Repository struct {
	Auth  repository.AuthRepository
	Minio repository.MinioRepository
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		Auth:  NewAuthPostgres(db),
		Minio: NewMinioPostgres(db),
	}
}
