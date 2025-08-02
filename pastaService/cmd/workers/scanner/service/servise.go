package scanner

import (
	"context"
	"fmt"
	domainrepo "pastebin/internal/domain/repository"
	domainservice "pastebin/internal/domain/service"
	"time"
)

type ScannerService struct {
	db domainrepo.ScannerDatabase
}

func NewScannerService(repo domainrepo.ScannerDatabase) domainservice.DBScanner {
	return &ScannerService{
		db: repo,
	}
}

func (s *ScannerService) GetExpiredPastas(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	objectIDs, err := s.db.GetExpiredPastas(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectIDs: %w", err)
	}
	return objectIDs, nil
}
