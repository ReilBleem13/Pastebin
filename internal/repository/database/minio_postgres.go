package database

import (
	"fmt"
	"log"
	"pastebin/internal/models"

	"github.com/jmoiron/sqlx"
)

type MinioPostgres struct {
	db *sqlx.DB
}

func NewMinioPostgres(db *sqlx.DB) *MinioPostgres {
	return &MinioPostgres{db: db}
}

func (m *MinioPostgres) CreatePasta(pasta models.Paste) error {
	_, err := m.db.Exec(fmt.Sprintf(
		"INSERT INTO %s (hash, user_id, key, size, created_at, expired_at) VALUES($1, $2, $3, $4, $5, $6)", pastasTables),
		pasta.Hash, pasta.UserID, pasta.Key, pasta.Size, pasta.CreatedAt, pasta.ExpiredAt)
	if err != nil {
		return err
	}
	return nil
}

func (m *MinioPostgres) GetLink(hash string) (string, error) {
	var storage_key string
	if err := m.db.Get(&storage_key, fmt.Sprintf("SELECT key FROM %s WHERE hash = $1", pastasTables), hash); err != nil {
		return "", fmt.Errorf("error: %v", err)
	}
	return storage_key, nil
}

func (m *MinioPostgres) GetAll(pasta *models.PasteWithData) error {
	var metadata models.Paste
	err := m.db.Get(&metadata, fmt.Sprintf(
		`	SELECT hash, user_id, key, size, created_at, expired_at 
			FROM %s WHERE key = $1`, pastasTables), pasta.Metadata.Key)
	if err != nil {
		log.Println(err)
		return err
	}
	pasta.Metadata = metadata
	return nil
}
