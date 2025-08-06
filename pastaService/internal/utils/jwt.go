package utils

import (
	"errors"
	"fmt"
	"pastebin/internal/config"
	customerrors "pastebin/internal/errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

var jwtKey []byte

func getJWTKey() []byte {
	if jwtKey == nil {
		jwtKey = []byte(config.GetConfig().App.JWTSecret)
	}
	return jwtKey
}

func VerifyAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, customerrors.ErrUnexpectedSignMethod
		}
		return getJWTKey(), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, customerrors.ErrTokenExpired
		}

		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, customerrors.ErrInvalidToken
		}
		return nil, fmt.Errorf("could not parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, customerrors.ErrInvalidToken
}

func GenerateTestJWT(userID int) (string, error) {
	accessClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(getJWTKey()))
	if err != nil {
		return "", err
	}
	return accessTokenString, nil
}
