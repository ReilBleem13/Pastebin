package service

import (
	"fmt"
	"pastebin/internal/models"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/redis"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"pastebin/pkg/validate"
	"time"
)

type DBMinioService struct {
	repo  database.Minio
	redis redis.Redis
}

func NewDBMinioService(repo database.Minio, redis redis.Redis) *DBMinioService {
	return &DBMinioService{repo: repo, redis: redis}
}

func (p *DBMinioService) CreatePasta(req dto.RequestCreatePasta, pasta *models.Paste) error {
	if !validate.CheckContains(validate.SupportedLanguages, req.Language) {
		return fmt.Errorf("invalid language format")
	}
	pasta.Language = req.Language

	if !validate.CheckContains(validate.SupportedVisibilities, req.Visibility) {
		return fmt.Errorf("invalid language format")
	}
	pasta.Visibility = req.Visibility

	key, ok := validate.SupportedTime[req.Expiration]
	if !ok {
		return fmt.Errorf("invalid time format")
	}
	pasta.ExpiresAt = time.Now().Add(time.Duration(key) * time.Millisecond)

	if req.Password == "" {
		return p.repo.CreatePasta(pasta)
	}

	hashPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}
	pasta.PasswordHash = hashPassword

	return p.repo.CreatePasta(pasta)
}

func (p *DBMinioService) GetLink(hash string) (string, error) {
	return p.repo.GetLink(hash)
}

func (p *DBMinioService) GetVisibility(hash string) (string, error) {
	return p.repo.GetVisibility(hash)
}

func (p *DBMinioService) GetPastaByUserID(userID int, hash string) error {
	return p.repo.GetPastaByUserID(userID, hash)
}

func (p *DBMinioService) AddViews(hash string) error {
	return p.repo.AddViews(hash)
}
