package service

import (
	domainrepo "authService/intenal/domain/repository"
	domainsrv "authService/intenal/domain/service"
	myerrors "authService/intenal/errors"
	"authService/intenal/utils"
	"authService/pkg/dto"
	"authService/pkg/retry"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theartofdevel/logging"
)

type AuthService struct {
	db     domainrepo.AuthRepository
	logger *logging.Logger
}

func NewScannerService(ctx context.Context, repo domainrepo.AuthRepository) domainsrv.Authorization {
	return &AuthService{
		db:     repo,
		logger: logging.L(ctx),
	}
}

func (a *AuthService) CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hashPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = hashPassword

	if !strings.Contains(user.Email, "@") {
		return myerrors.ErrInvalidEmailFormat
	}
	if len(user.Password) < 10 {
		return myerrors.ErrShortPassword
	}

	return retry.Retry(ctx, func() error {
		return a.db.CreateUser(ctx, user)
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), a.logger)

}

func (a *AuthService) CheckLogin(ctx context.Context, request *dto.LoginUser) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var hashPassword string
	err := retry.Retry(ctx, func() error {
		var err error
		hashPassword, err = a.db.GetHashPassword(ctx, request.Email)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), a.logger)
	if err != nil {
		return fmt.Errorf("failed to get password hash: %w", err)
	}

	if !utils.CheckPasswordHash(request.Password, hashPassword) {
		return myerrors.ErrWrongPassword
	}
	return nil
}

func (a *AuthService) GenerateToken(ctx context.Context, request *dto.LoginUser) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var id int
	err := retry.Retry(ctx, func() error {
		var err error
		id, err = a.db.GetUserIDByEmail(ctx, request.Email)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), a.logger)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user id by email: %w", err)
	}
	accessToken, refreshToken, err := utils.GenerateTokens(id)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate token: %w", err)
	}
	return accessToken, refreshToken, nil
}

func (a *AuthService) RefreshAccessToken(refreshToken string) (string, error) {
	return utils.RefreshAccessToken(refreshToken)
}
