package repository

import (
	domain "authService/intenal/domain/repository"
	myerrors "authService/intenal/errors"
	"authService/pkg/dto"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	usersTables = "users"
)

type AuthRepository struct {
	db *sqlx.DB
}

func NewAuthRepository(db *sqlx.DB) domain.AuthRepository {
	return &AuthRepository{db: db}
}

func (a *AuthRepository) CreateUser(ctx context.Context, user *dto.RequestNewUser) error {
	fmt.Println(user.Name, user.Email, user.Password)
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, email, password_hash) VALUES($1, $2, $3)", usersTables),
		user.Name, user.Email, user.Password)

	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return myerrors.ErrUserAlreadyExist
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (a *AuthRepository) GetHashPassword(ctx context.Context, email string) (string, error) {
	var hashPassword string
	err := a.db.GetContext(ctx, &hashPassword, fmt.Sprintf("SELECT password_hash FROM %s WHERE email = $1", usersTables), email)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", myerrors.ErrUserNotFound
		}
		return "", err
	}
	return hashPassword, nil
}

func (a *AuthRepository) GetUserIDByEmail(ctx context.Context, email string) (int, error) {
	var userID int
	err := a.db.GetContext(ctx, &userID, fmt.Sprintf("SELECT id FROM %s WHERE email = $1", usersTables), email)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, myerrors.ErrUserNotFound
		}
		return 0, err
	}
	return userID, nil
}
