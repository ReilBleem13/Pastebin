package scanner

import (
	"context"
	domainrepo "pastebin/internal/domain/repository"
	domainservice "pastebin/internal/domain/service"
	"pastebin/pkg/logging"
	"time"
)

type ScannerService struct {
	db     domainrepo.ScannerDatabase
	logger *logging.Logger
}

func NewScannerService(repo domainrepo.ScannerDatabase, logger *logging.Logger) domainservice.DBScanner {
	return &ScannerService{
		db:     repo,
		logger: logger,
	}
}

func (s *ScannerService) GetExpiredPastas(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.db.GetExpiredPastas(ctx)
}
