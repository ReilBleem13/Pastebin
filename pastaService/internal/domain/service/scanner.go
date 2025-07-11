package domain

import "context"

type DBScanner interface {
	GetExpiredPastas(ctx context.Context) ([]string, error)
}
