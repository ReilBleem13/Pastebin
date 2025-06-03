package database

import "github.com/jmoiron/sqlx"

type Authorization interface{}

type Minio interface {
	CreateLink(objectID, hash string) error
	GetLink(hash string) (string, error)
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
