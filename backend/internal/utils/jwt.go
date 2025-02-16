package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService interface {
	GenerateToken(userID string) (*string, error)
	GenerateRefreshToken(userID string) (*string, error)
	ValidateToken(token string) (*string, error)
}

type jwtService struct {
	secretKey            string
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

func NewJWTService(secretKey string, accessTokenDuration time.Duration, refreshTokenDuration time.Duration) JWTService {
	return &jwtService{
		secretKey:            secretKey,
		accessTokenDuration:  accessTokenDuration,
		refreshTokenDuration: refreshTokenDuration,
	}
}

func (s *jwtService) GenerateToken(userID string) (*string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"iat":     time.Now().Unix(),
		"iss":     "neobase-ai",
		"exp":     time.Now().Add(s.accessTokenDuration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		return nil, err
	}
	return &tokenString, nil
}

func (s *jwtService) GenerateRefreshToken(userID string) (*string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"iat":     time.Now().Unix(),
		"iss":     "neobase-ai",
		"exp":     time.Now().Add(s.refreshTokenDuration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		return nil, err
	}
	return &tokenString, nil
}

func (s *jwtService) ValidateToken(tokenString string) (*string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := claims["user_id"].(string)
		expiresAt := int64(claims["exp"].(float64))
		if expiresAt < time.Now().Unix() {
			return nil, errors.New("token has expired")
		}
		return &userID, nil
	}

	return nil, err
}
