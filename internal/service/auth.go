package service

import "pastebin/internal/repository/database"

type AuthService struct {
	repo database.Authorization
}

func NewAuthService(repo database.Authorization) *AuthService {
	return &AuthService{
		repo: repo,
	}
}
