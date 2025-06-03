package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type MinioPostgres struct {
	db *sqlx.DB
}

func NewMinioPostgres(db *sqlx.DB) *MinioPostgres {
	return &MinioPostgres{db: db}
}

func (m *MinioPostgres) CreateLink(objectID, hash string) error {
	_, err := m.db.Exec(fmt.Sprintf("INSERT INTO %s (object_name, object_hash) VALUES($1, $2)", linksTables), objectID, hash)
	if err != nil {
		return err
	}
	return nil
}

func (m *MinioPostgres) GetLink(hash string) (string, error) {
	var url string
	if err := m.db.Get(&url, fmt.Sprintf("SELECT object_name FROM %s WHERE object_hash = $1", linksTables), hash); err != nil {
		return "", err
	}
	return url, nil
}
