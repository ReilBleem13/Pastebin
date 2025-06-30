package database

import (
	"context"
	"fmt"
	domain "pastebin/internal/domain/repository"
	"pastebin/pkg/dto"

	"github.com/jmoiron/sqlx"
)

type authDatabase struct {
	db *sqlx.DB
}

func NewAuthDatabase(db *sqlx.DB) domain.AuthDatabase {
	return &authDatabase{
		db: db,
	}
}

func (a *authDatabase) CreateUser(ctx context.Context, user *dto.RequestNewUser) error {
	fmt.Println(user.Name, user.Email, user.Password)
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, email, password_hash) VALUES($1, $2, $3)", usersTables),
		user.Name, user.Email, user.Password)

	if err != nil {
		return err
	}
	return nil
}

func (a *authDatabase) GetHashPassword(ctx context.Context, email string) (string, error) {
	var hashPassword string
	err := a.db.GetContext(ctx, &hashPassword, fmt.Sprintf("SELECT password_hash FROM %s WHERE email = $1", usersTables), email)
	if err != nil {
		return "", err
	}
	return hashPassword, nil
}

func (a *authDatabase) GetUserIDByEmail(ctx context.Context, email string) (int, error) {
	var userID int
	err := a.db.GetContext(ctx, &userID, fmt.Sprintf("SELECT id FROM %s WHERE email = $1", usersTables), email)
	if err != nil {
		return 0, err
	}
	return userID, nil
}
