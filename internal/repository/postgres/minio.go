package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pastebin/internal/models"

	"github.com/jmoiron/sqlx"
)

type minioPostgres struct {
	db *sqlx.DB
}

func NewMinioPostgres(db *sqlx.DB) *minioPostgres {
	return &minioPostgres{db: db}
}

func (m *minioPostgres) CreateMetadata(ctx context.Context, pasta *models.Paste) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (hash, key, user_id, size, language, visibility, password_hash, created_at, expires_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)", pastasTables),
		pasta.Hash, pasta.Key, pasta.UserID, pasta.Size, pasta.Language, pasta.Visibility, pasta.PasswordHash, pasta.CreatedAt, pasta.ExpiresAt)
	if err != nil {
		return err
	}
	return nil
}

func (m *minioPostgres) GetKey(ctx context.Context, hash string) (string, error) {
	var key string
	if err := m.db.GetContext(ctx, &key, fmt.Sprintf("SELECT key FROM %s WHERE hash = $1", pastasTables), hash); err != nil {
		return "", fmt.Errorf("error: %v", err)
	}
	return key, nil
}

func (m *minioPostgres) GetVisibility(ctx context.Context, hash string) (string, error) {
	var visibility string

	err := m.db.GetContext(ctx, &visibility, fmt.Sprintf("SELECT visibility FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return "", err
	}
	return visibility, nil
}

func (m *minioPostgres) GetMetadata(ctx context.Context, pasta *models.Paste) error {
	err := m.db.GetContext(ctx, pasta, fmt.Sprintf(
		`	SELECT hash, key, user_id, size, language, visibility, views, created_at, expires_at 
			FROM %s WHERE key = $1`, pastasTables), pasta.Key)
	if err != nil {
		return err
	}
	return nil
}

func (m *minioPostgres) GetPassword(ctx context.Context, hash string) (string, error) {
	var hashPassword string

	err := m.db.GetContext(ctx, &hashPassword, fmt.Sprintf("SELECT password_hash FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("password_hash is empty")
		}
		return "", err
	}
	return hashPassword, nil
}

func (m *minioPostgres) AddViews(ctx context.Context, hash string) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET views = views+1 WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return err
	}
	return nil
}

func (m *minioPostgres) CheckPermission(ctx context.Context, userID int, hash string) (bool, error) {
	var password_hash string

	err := m.db.GetContext(ctx, &password_hash, fmt.Sprintf("SELECT password_hash FROM %s WHERE user_id = $1 AND hash = $2", pastasTables), userID, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, errors.New("failed to fetch password_hash")
		}
		return false, err
	}
	return password_hash != "", nil
}

func (m *minioPostgres) GetKeys(ctx context.Context, userID int) ([]string, error) {
	var keys []string
	err := m.db.GetContext(ctx, &keys, fmt.Sprintf("SELECT key FROM %s WHERE user_id = $1", pastasTables), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, errors.New("pastas is empty")
		}
		return []string{}, err
	}
	return keys, nil
}

func (m *minioPostgres) DeleteMetadata(ctx context.Context, hash string) (string, error) {
	var key string
	err := m.db.GetContext(ctx, &key, fmt.Sprintf("DELETE FROM %s WHERE hash = $1 RETURNING key", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("metadata not found for hash: %s", hash)
		}
		return "", err
	}
	return key, nil
}
