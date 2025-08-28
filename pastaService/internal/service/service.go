package service

import (
	domain "pastebin/internal/domain/service"
	"pastebin/internal/repository"

	"github.com/theartofdevel/logging"
	"golang.org/x/net/context"
)

type Service struct {
	Pasta domain.Pasta
}

func NewService(ctx context.Context, repo *repository.Repository) *Service {
	return &Service{
		Pasta: NewPastaService(repo, logging.L(ctx)),
	}
}
