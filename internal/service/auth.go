package service

import (
	"fmt"
	"pastebin/internal/repository/database"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"
)

type AuthService struct {
	repo database.Authorization
}

func NewAuthService(repo database.Authorization) *AuthService {
	return &AuthService{
		repo: repo,
	}
}

func (a *AuthService) CreateNewUser(user *dto.RequestNewUser) error {
	if !strings.Contains(user.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	if len(user.Password) < 10 {
		return fmt.Errorf("password is short. min: lenght = 10")
	}
	return a.repo.CreateUser(user)
}

func (a *AuthService) CheckLogin(request *dto.LoginUser) error {
	hashPassword, err := a.repo.GetHashPassword(request.Email)
	if err != nil {
		return err
	}

	if !utils.CheckPasswordHash(request.Password, hashPassword) {
		return fmt.Errorf("wrong password")
	}

	return nil
}

func (a *AuthService) GenerateToken(request *dto.LoginUser) (string, error) {
	id, err := a.repo.GetUserIDByEmail(request.Email)
	if err != nil {
		return "", err
	}
	accessToke, err := utils.GenerateToken(id)
	if err != nil {
		return "", err
	}

	return accessToke, nil
}
