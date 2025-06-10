package database

import (
	"database/sql"
	"errors"
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

func (m *MinioPostgres) CreatePasta(pasta *models.Paste) error {
	_, err := m.db.Exec(fmt.Sprintf(
		"INSERT INTO %s (hash, key, user_id, size, language, visibility, password_hash, created_at, expires_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)", pastasTables),
		pasta.Hash, pasta.Key, pasta.UserID, pasta.Size, pasta.Language, pasta.Visibility, pasta.PasswordHash, pasta.CreatedAt, pasta.ExpiresAt)
	if err != nil {
		return err
	}
	log.Printf("user_id: %v", pasta.UserID)
	return nil
}

func (m *MinioPostgres) GetLink(hash string) (string, error) {
	var storage_key string
	if err := m.db.Get(&storage_key, fmt.Sprintf("SELECT key FROM %s WHERE hash = $1", pastasTables), hash); err != nil {
		return "", fmt.Errorf("error: %v", err)
	}
	return storage_key, nil
}

func (m *MinioPostgres) GetVisibility(hash string) (string, error) {
	var visibility string

	err := m.db.Get(&visibility, fmt.Sprintf("SELECT visibility FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return "", err
	}
	return visibility, nil
}

func (m *MinioPostgres) GetAll(pasta *models.Paste) error {
	log.Println(200)
	err := m.db.Get(pasta, fmt.Sprintf(
		`	SELECT hash, key, user_id, size, language, visibility, views, created_at, expires_at 
			FROM %s WHERE key = $1`, pastasTables), pasta.Key)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println(100, pasta.Visibility, pasta.Language)
	return nil
}

func (m *MinioPostgres) GetPastaByUserID(hash string) error {
	var id int
	err := m.db.Get(&id, fmt.Sprintf("SELECT id FROM %s WHERE hash = $1", pastasTables), hash)
	if err == sql.ErrNoRows {
		return fmt.Errorf("pasta not found")
	}
	return nil
}

func (m *MinioPostgres) GetHashPassword(hash string) (string, error) {
	var hashPassword string

	err := m.db.Get(&hashPassword, fmt.Sprintf("SELECT password_hash FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("password_hash is empty")
		}
		return "", err
	}
	return hashPassword, nil
}

func (m *MinioPostgres) AddViews(hash string) error {
	_, err := m.db.Exec(fmt.Sprintf("UPDATE %s SET views = views+1 WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return err
	}
	return nil
}

///

func (m *MinioPostgres) CheckPermission(userID int, hash string) (string, error) {
	var password_hash string

	err := m.db.Get(&password_hash, fmt.Sprintf("SELECT password_hash FROM %s WHERE user_id = $1 AND hash = $2", pastasTables), userID, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("failed to fetch password_hash")
		}
		return "", err
	}
	return password_hash, nil
}

func (m *MinioPostgres) DeleteMetadata(hash string) (string, error) {
	var key string
	err := m.db.Get(&key, fmt.Sprintf("DELETE FROM %s WHERE hash = $1 RETURNING key", pastasTables), hash)
	if err != nil {
		return "", err
	}
	return key, nil
} // обработку ошибки
