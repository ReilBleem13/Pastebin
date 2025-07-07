package service

import (
	"context"
	"fmt"
	domainrepo "pastebin/internal/domain/repository"
	domainservice "pastebin/internal/domain/service"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/repository"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"pastebin/pkg/retry"
	"strings"
	"time"
)

type AuthService struct {
	db domainrepo.AuthDatabase
}

func NewAuthService(repo *repository.Repository) domainservice.Authorization {
	return &AuthService{
		db: repo.Database.Auth(),
	}
}

func (a *AuthService) CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if !strings.Contains(user.Email, "@") {
		return customerrors.ErrInvalidEmailFormat
	}
	if len(user.Password) < 10 {
		return customerrors.ErrShortPassword
	}
	err := retry.WithRetry(ctx, func() error {
		return a.db.CreateUser(ctx, user)
	}, retry.IsRetryableErrorDatabase)
	return err
}

func (a *AuthService) CheckLogin(ctx context.Context, request *dto.LoginUser) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var hashPassword string
	err := retry.WithRetry(ctx, func() error {
		var err error
		hashPassword, err = a.db.GetHashPassword(ctx, request.Email)
		return err
	}, retry.IsRetryableErrorDatabase)
	if err != nil {
		return fmt.Errorf("failed to get password_hash: %w", err)
	}

	if !utils.CheckPasswordHash(request.Password, hashPassword) {
		return customerrors.ErrWrongPassword
	}
	return nil
}

func (a *AuthService) GenerateToken(ctx context.Context, request *dto.LoginUser) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var id int
	err := retry.WithRetry(ctx, func() error {
		var err error
		id, err = a.db.GetUserIDByEmail(ctx, request.Email)
		return err
	}, retry.IsRetryableErrorDatabase)
	if err != nil {
		return "", fmt.Errorf("failed to get user id by email: %w", err)
	}
	accessToke, err := utils.GenerateToken(id)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return accessToke, nil
}
