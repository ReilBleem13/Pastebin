package service

import (
	"errors"
	"fmt"
	"pastebin/internal/models"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/redis"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"pastebin/pkg/validate"
	"strings"
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
	if req.Language != nil {
		if !validate.CheckContains(validate.SupportedLanguages, *req.Language) {
			return fmt.Errorf("invalid language format: %v", pasta.Language)
		} else {
			pasta.Language = req.Language
		}
	} else {
		pasta.Language = prtSrt("plaintext")
	}

	if req.Visibility != nil {
		if !validate.CheckContains(validate.SupportedVisibilities, *req.Visibility) {
			return fmt.Errorf("invalid visibility format: %v", pasta.Visibility)
		} else {
			pasta.Visibility = req.Visibility
		}
	} else {
		pasta.Visibility = prtSrt("public")
	}

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

func (p *DBMinioService) CheckPermission(userID int, hash string) (bool, error) {
	password_hash, err := p.repo.CheckPermission(userID, hash)
	if err != nil {
		if strings.Contains(err.Error(), "failed to fetch id") {
			return false, errors.New("no rights")
		}
		return false, err
	}
	return password_hash != "", nil
}

func (p *DBMinioService) CheckPastaPassword(password, hash string) error {
	password_hash, err := p.repo.GetHashPassword(hash)
	if err != nil {
		if strings.Contains(err.Error(), "password_hash is empty") {
			return nil
		}
	}

	if !utils.CheckPasswordHash(password, password_hash) {
		return fmt.Errorf("wrong password")
	}
	return nil
}

func (p *DBMinioService) GetHashPassword(hash string) (string, error) {
	password_hash, err := p.repo.GetHashPassword(hash)
	if err != nil {
		if strings.Contains(err.Error(), "password_hash is empty") {
			return "", fmt.Errorf("pasta without password")
		}
	}
	return password_hash, nil
} // удалить

func (p *DBMinioService) GetLink(hash string) (string, error) {
	return p.repo.GetLink(hash)
}

func (p *DBMinioService) GetVisibility(hash string) (string, error) {
	return p.repo.GetVisibility(hash)
}

func (p *DBMinioService) GetPastaByUserID(hash string) error {
	return p.repo.GetPastaByUserID(hash)
}

func (p *DBMinioService) AddViews(hash string) error {
	return p.repo.AddViews(hash)
}

func prtSrt(s string) *string {
	return &s
}
