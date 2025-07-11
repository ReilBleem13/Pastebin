package service

import (
	domainrepo "authService/intenal/domain/repository"
	domainsrv "authService/intenal/domain/service"
	myerrors "authService/intenal/errors"
	"authService/pkg/dto"
	"authService/pkg/hash"
	"authService/pkg/jwt"
	"authService/pkg/logging"
	"authService/pkg/retry"
	"context"
	"fmt"
	"strings"
	"time"
)

type AuthService struct {
	db     domainrepo.AuthRepository
	logger *logging.Logger
}

func NewScannerService(repo domainrepo.AuthRepository, logger *logging.Logger) domainsrv.Authorization {
	return &AuthService{
		db:     repo,
		logger: logger,
	}
}

func (a *AuthService) CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if !strings.Contains(user.Email, "@") {
		return myerrors.ErrInvalidEmailFormat
	}
	if len(user.Password) < 10 {
		return myerrors.ErrShortPassword
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

	if !hash.CheckPasswordHash(request.Password, hashPassword) {
		return myerrors.ErrWrongPassword
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
	accessToke, err := jwt.GenerateToken(id)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return accessToke, nil
}
