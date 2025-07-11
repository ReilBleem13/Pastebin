package service

import (
	domain "pastebin/internal/domain/service"
	"pastebin/internal/repository"
	"pastebin/pkg/logging"
)

type Service struct {
	Pasta domain.Pasta
}

func NewService(repo *repository.Repository, logger *logging.Logger) *Service {
	return &Service{
		Pasta: NewPastaService(repo, logger),
	}
}
