package service

import (
	"context"
	"fmt"
	domainrepo "pastebin/internal/domain/repository"
	domainservice "pastebin/internal/domain/service"
	"pastebin/internal/repository"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"
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
	if !strings.Contains(user.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	if len(user.Password) < 10 {
		return fmt.Errorf("password is short. min: lenght = 10")
	}
	return a.db.CreateUser(ctx, user)
}

func (a *AuthService) CheckLogin(ctx context.Context, request *dto.LoginUser) error {
	hashPassword, err := a.db.GetHashPassword(ctx, request.Email)
	if err != nil {
		return err
	}

	if !utils.CheckPasswordHash(request.Password, hashPassword) {
		return fmt.Errorf("wrong password")
	}

	return nil
}

func (a *AuthService) GenerateToken(ctx context.Context, request *dto.LoginUser) (string, error) {
	id, err := a.db.GetUserIDByEmail(ctx, request.Email)
	if err != nil {
		return "", err
	}
	accessToke, err := utils.GenerateToken(id)
	if err != nil {
		return "", err
	}

	return accessToke, nil
}
