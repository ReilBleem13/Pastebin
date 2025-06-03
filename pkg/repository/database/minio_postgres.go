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

func (m *MinioPostgres) CreateLink(objectID, url string) error {
	_, err := m.db.Exec(fmt.Sprintf("INSERT INTO %s (object_id, url_name) VALUES($1, $2)", linksTables), objectID, url)
	if err != nil {
		return err
	}
	return nil
}

func (m *MinioPostgres) GetLink(objectID string) (string, error) {
	var url string
	if err := m.db.Get(&url, fmt.Sprintf("SELECT url_name FROM %s WHERE object_id", linksTables)); err != nil {
		return "", err
	}
	return url, nil
}
