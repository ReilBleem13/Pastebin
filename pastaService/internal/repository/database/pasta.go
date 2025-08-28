package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	domain "pastebin/internal/domain/repository"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type pastaDatabase struct {
	db *sqlx.DB
}

func NewPastaDatabase(db *sqlx.DB) domain.PastaDatabase {
	return &pastaDatabase{db: db}
}

func (m *pastaDatabase) Create(ctx context.Context, pasta *models.Pasta) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (hash, object_id, user_id, size, language, visibility, password_hash, created_at, expires_at, expire_after_read) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)", pastasTables),
		pasta.Hash, pasta.ObjectID, pasta.UserID, pasta.Size, pasta.Language, pasta.Visibility, pasta.PasswordHash, pasta.CreatedAt, pasta.ExpiresAt, pasta.ExpireAfterRead)
	if err != nil {
		return err
	}
	return nil
}

func (m *pastaDatabase) GetKey(ctx context.Context, hash string) (string, error) {
	var object_id string
	if err := m.db.GetContext(ctx, &object_id, fmt.Sprintf("SELECT object_id FROM %s WHERE hash = $1", pastasTables), hash); err != nil {
		return "", fmt.Errorf("error: %v", err)
	}
	return object_id, nil
}

func (m *pastaDatabase) GetVisibility(ctx context.Context, hash string) (string, error) {
	var visibility string

	err := m.db.GetContext(ctx, &visibility, fmt.Sprintf("SELECT visibility FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", customerrors.ErrPastaNotFound
		}
		return "", err
	}
	return visibility, nil
}

func (m *pastaDatabase) GetMetadata(ctx context.Context, hash string) (*models.Pasta, error) {
	pasta := models.Pasta{}
	err := m.db.GetContext(ctx, &pasta, fmt.Sprintf(
		`	SELECT hash, object_id, user_id, size, language, visibility, views, created_at, expires_at, expire_after_read
			FROM %s WHERE hash = $1`, pastasTables), hash)
	if err != nil {
		return nil, err
	}

	log.Printf("metadata from db: %+v\n", pasta)
	return &pasta, nil
}

func (m *pastaDatabase) GetPassword(ctx context.Context, hash string) (string, error) {
	var hashPassword string

	err := m.db.GetContext(ctx, &hashPassword, fmt.Sprintf("SELECT password_hash FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", customerrors.ErrPasswordIsEmpty
		}
		return "", err
	}
	return hashPassword, nil
}

func (m *pastaDatabase) GetHash(ctx context.Context, objectID string) (string, error) {
	var hash string
	err := m.db.GetContext(ctx, &hash, fmt.Sprintf("SELECT hash FROM %s WHERE object_id = $1", pastasTables), objectID)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (m *pastaDatabase) GetUserID(ctx context.Context, hash string) (int, error) {
	var userID int
	err := m.db.GetContext(ctx, &userID, fmt.Sprintf("SELECT user_id FROM %s WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (m *pastaDatabase) GetPublicHashs(ctx context.Context, objectID []string) ([]string, error) {
	var hashs []string
	err := m.db.SelectContext(ctx, &hashs, fmt.Sprintf("SELECT hash FROM %s WHERE object_id = ANY($1) AND visibility != 'private'", pastasTables), pq.Array(objectID))
	if err != nil {
		return nil, err
	}
	return hashs, nil
}

func (m *pastaDatabase) AddViews(ctx context.Context, hash string) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET views = views+1 WHERE hash = $1", pastasTables), hash)
	if err != nil {
		return err
	}
	return nil
}

func (m *pastaDatabase) CheckPermission(ctx context.Context, userID int, hash string) (bool, error) {
	var password_hash string

	err := m.db.GetContext(ctx, &password_hash, fmt.Sprintf("SELECT password_hash FROM %s WHERE user_id = $1 AND hash = $2", pastasTables), userID, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, customerrors.ErrFailedFetchPassword
		}
		return false, err
	}
	return password_hash != "", nil
}

func (m *pastaDatabase) GetKeys(ctx context.Context, userID int) ([]string, error) {
	var objectIDs []string
	err := m.db.GetContext(ctx, &objectIDs, fmt.Sprintf("SELECT object_id FROM %s WHERE user_id = $1", pastasTables), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, customerrors.ErrPastaNotFound
		}
		return []string{}, err
	}
	return objectIDs, nil
}

func (m *pastaDatabase) DeleteMetadata(ctx context.Context, hash string) (string, error) {
	var key string
	err := m.db.GetContext(ctx, &key, fmt.Sprintf("DELETE FROM %s WHERE hash = $1 RETURNING object_id", pastasTables), hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", customerrors.ErrPastaNotFound
		}
		return "", err
	}
	return key, nil
}

func (m *pastaDatabase) GetKeysExpiredPasta(ctx context.Context) ([]string, error) {
	var keys []string
	err := m.db.SelectContext(ctx, &keys, fmt.Sprintf("SELECT key FROM %s WHERE expires_at < NOW()", pastasTables))
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, errors.New("expired pasta not found")
		}
		return nil, err
	}
	return keys, nil
}

func (m *pastaDatabase) DeleteExpiredPasta(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE key = ANY($1)", pastasTables)
	_, err := m.db.ExecContext(ctx, query, pq.Array(keys))
	if err != nil {
		return fmt.Errorf("failed to delete expired pasta: %w", err)
	}

	return nil
}

func (m *pastaDatabase) IsPastaExists(ctx context.Context, hash string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE hash = $1)", pastasTables)

	var exists bool
	if err := m.db.GetContext(ctx, &exists, query, hash); err != nil {
		return exists, err
	}
	return exists, nil
}

func (m *pastaDatabase) IsPastaExistsByObjectID(ctx context.Context, objectID string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE object_id = $1)", pastasTables)

	var exists bool
	if err := m.db.GetContext(ctx, &exists, query, objectID); err != nil {
		return exists, err
	}
	return exists, nil
}

func (m *pastaDatabase) IsAccessPermission(ctx context.Context, userID int, hash string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1 AND hash = $2)", pastasTables)

	var exists bool
	if err := m.db.GetContext(ctx, &exists, query, userID, hash); err != nil {
		return exists, err
	}
	return exists, nil
}

func (m *pastaDatabase) PaginateOnlyPublic(ctx context.Context, limit, offset int) ([]string, error) {
	query := `
		SELECT object_id 
		FROM pastas 
		WHERE object_id LIKE 'public:%' AND (password_hash IS NULL OR password_hash = '')
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	var objectIDs []string
	err := m.db.SelectContext(ctx, &objectIDs, query, limit, offset)
	if err != nil {
		return nil, err
	}
	return objectIDs, nil
}

func (m *pastaDatabase) PaginateFavorites(ctx context.Context, limit, offset, userID int) ([]string, error) {
	query := `
		SELECT p.object_id
		FROM pastas p
		JOIN favorites f ON p.id = f.pasta_id
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`
	var objectIDs []string
	err := m.db.SelectContext(ctx, &objectIDs, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return objectIDs, nil

}

func (m *pastaDatabase) PaginateOnlyByUserID(ctx context.Context, limit, offset, userID int) ([]string, error) {
	query := `
		SELECT object_id 
		FROM pastas 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	var objectIDs []string
	err := m.db.SelectContext(ctx, &objectIDs, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return objectIDs, nil
}

func (m *pastaDatabase) GetManyMetadataPublic(ctx context.Context, objectID []string) ([]models.Pasta, error) {
	var metadatas []models.Pasta

	query := `
		SELECT hash, object_id, user_id, size, language, visibility, views, created_at, expires_at 	
		FROM pastas
		WHERE object_id = ANY($1) AND password_hash = ''
	`

	err := m.db.SelectContext(ctx, &metadatas, query, pq.Array(objectID))
	if err != nil {
		return nil, err
	}
	fmt.Printf("Длина слайса objectIDs: %d. Кол-во полученных паст: %d", len(objectID), len(metadatas))
	return metadatas, nil
}

func (m *pastaDatabase) GetManyMetadata(ctx context.Context, objectID []string) ([]models.Pasta, error) {
	var metadatas []models.Pasta

	query := `
		SELECT hash, object_id, user_id, size, language, visibility, views, created_at, expires_at 	
		FROM pastas
		WHERE object_id = ANY($1)`

	err := m.db.SelectContext(ctx, &metadatas, query, pq.Array(objectID))
	if err != nil {
		return nil, err
	}
	return metadatas, nil
}

func (m *pastaDatabase) GetExpireAfterReadField(ctx context.Context, hash string) (bool, error) {
	var isExpireAfterRead bool

	query := `
		SELECT expire_after_read 
		FROM pastas
		WHERE hash = $1	
	`

	err := m.db.GetContext(ctx, &isExpireAfterRead, query, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("error: %w, %v", customerrors.ErrPastaNotFound, err)
		}
		return false, err
	}
	return isExpireAfterRead, nil
}

func (m *pastaDatabase) UpdateSizeAndReturnAll(ctx context.Context, hash string, size int) (*models.Pasta, error) {
	var metadata models.Pasta

	tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE pastas SET size = $1 WHERE hash = $2
	`
	_, err = tx.ExecContext(ctx, query, size, hash)
	if err != nil {
		return nil, err
	}

	query = `
		SELECT hash, object_id, user_id, size, language, visibility, views, expire_after_read, created_at, expires_at
		FROM pastas
		WHERE hash = $1	
	`
	if err = tx.GetContext(ctx, &metadata, query, hash); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (m *pastaDatabase) Favorite(ctx context.Context, hash string, id int) error {
	timeNow := time.Now()

	query := `
		INSERT INTO favorites(user_id, pasta_id, created_at)
		VALUES($1, (SELECT id FROM pastas WHERE hash = $2), $3)
	`

	_, err := m.db.ExecContext(ctx, query, id, hash, timeNow)
	if err != nil {
		return fmt.Errorf("failed to insert new favorite pasta: %w", err)
	}
	return nil
}

func (m *pastaDatabase) GetFavoriteAndCheckUser(ctx context.Context, userID, favoriteID int) (string, error) {
	query := `
		SELECT p.hash
		FROM favorites f
		JOIN pastas p ON f.pasta_id = p.id
		WHERE f.id = $1 AND f.user_id = $2
	`

	var hash string
	err := m.db.QueryRowContext(ctx, query, favoriteID, userID).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", customerrors.ErrNotAllowed
		}
		return "", err
	}
	return hash, nil
}

func (m *pastaDatabase) DeleteFavorite(ctx context.Context, userID, favoriteID int) error {
	query := `
		DELETE FROM favorites 
		WHERE id = $1 AND user_id = $2
		RETURNING id
	`

	var deletedID int
	err := m.db.GetContext(ctx, &deletedID, query, favoriteID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return customerrors.ErrNotAllowed
		}
		return err
	}

	return nil
}

func (m *pastaDatabase) GetExpiredPastas(ctx context.Context) ([]string, error) {
	var objectIDs []string
	timeNow := time.Now().UTC()
	query := `
		SELECT object_id FROM pastas WHERE expires_at < $1
	`

	err := m.db.SelectContext(ctx, &objectIDs, query, timeNow)
	if err != nil {
		return nil, err
	}
	return objectIDs, nil
}

func (m *pastaDatabase) DeletePastas(ctx context.Context, objectIDs []string) error {
	query := `DELETE FROM pastas WHERE object_id = ANY($1)`
	_, err := m.db.ExecContext(ctx, query, pq.Array(objectIDs))
	return err
}
