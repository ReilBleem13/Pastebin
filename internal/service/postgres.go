package service

import (
	"context"
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
	repo  database.MinioMetadata
	redis redis.Redis
}

func NewDBMinioService(repo database.MinioMetadata, redis redis.Redis) *DBMinioService {
	return &DBMinioService{repo: repo, redis: redis}
}

func (p *DBMinioService) CreatePasta(ctx context.Context, req dto.RequestCreatePasta, pasta *models.Paste) error {
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

	if req.Expiration != nil {
		key, ok := validate.SupportedTime[*req.Expiration]
		if !ok {
			return fmt.Errorf("invalid time format")
		}
		pasta.ExpiresAt = time.Now().Add(time.Duration(key) * time.Millisecond)
	} else {
		pasta.ExpiresAt = time.Now().Add(time.Duration(validate.SupportedTime["1h"]) * time.Millisecond)
	}

	if req.Password == nil {
		return p.repo.CreateMetadata(ctx, pasta)
	}

	hashPassword, err := utils.HashPassword(*req.Password)
	if err != nil {
		return err
	}
	pasta.PasswordHash = hashPassword
	return p.repo.CreateMetadata(ctx, pasta)
}

func (p *DBMinioService) CheckPrivatePermission(ctx context.Context, userID int, hash string) (bool, error) {
	hasPassword, err := p.repo.CheckPermission(ctx, userID, hash)
	if err != nil {
		if strings.Contains(err.Error(), "failed to fetch password_hash") {
			return false, errors.New("no rights")
		}
		return false, err
	}
	return hasPassword, nil
}

func (p *DBMinioService) CheckPublicPermission(ctx context.Context, hash string) (bool, error) {
	password_hash, err := p.repo.GetPassword(ctx, hash)
	if err != nil {
		if strings.Contains(err.Error(), "password_hash is empty") {
			return false, nil
		}
		return false, err
	}
	return password_hash != "", nil
}

func (p *DBMinioService) CheckPastaPassword(ctx context.Context, password, hash string) error {
	password_hash, err := p.repo.GetPassword(ctx, hash)
	if err != nil {
		if strings.Contains(err.Error(), "password_hash is empty") {
			return nil
		}
		return err
	}

	if !utils.CheckPasswordHash(password, password_hash) {
		return fmt.Errorf("wrong password")
	}
	return nil
}

func (p *DBMinioService) GetLink(ctx context.Context, hash string) (string, error) {
	return p.repo.GetKey(ctx, hash)
}

func (p *DBMinioService) GetVisibility(ctx context.Context, hash string) (string, error) {
	return p.repo.GetVisibility(ctx, hash)
}

func (p *DBMinioService) AddViews(ctx context.Context, hash string) error {
	return p.repo.AddViews(ctx, hash)
}

func prtSrt(s string) *string {
	return &s
}
