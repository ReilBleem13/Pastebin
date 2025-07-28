package scanner

import (
	"context"
	"database/sql"
	domain "pastebin/internal/domain/repository"
	customerrors "pastebin/internal/errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type ScannerDatabase struct {
	db *sqlx.DB
}

func NewScannerDatabase(db *sqlx.DB) domain.ScannerDatabase {
	return &ScannerDatabase{db: db}
}

func (s *ScannerDatabase) GetExpiredPastas(ctx context.Context) ([]string, error) {
	var objectIDs []string
	timeNow := time.Now()
	query := `
		SELECT object_id FROM pastas WHERE expires_at < $1
	`

	err := s.db.SelectContext(ctx, &objectIDs, query, timeNow)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, customerrors.ErrPastaNotFound
		}
		return nil, err
	}
	return objectIDs, nil
}
