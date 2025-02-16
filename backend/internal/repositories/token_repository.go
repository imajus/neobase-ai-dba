package repositories

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/pkg/redis"
	"time"
)

type TokenRepository interface {
	StoreRefreshToken(userID string, refreshToken string) error
	ValidateRefreshToken(userID string, refreshToken string) bool
	DeleteRefreshToken(userID string, refreshToken string) error
	BlacklistToken(token string, expiresAt time.Duration) error
	IsTokenBlacklisted(token string) bool
}

type tokenRepository struct {
	redis redis.IRedisRepositories
}

func NewTokenRepository(redis redis.IRedisRepositories) TokenRepository {
	return &tokenRepository{
		redis: redis,
	}
}

func (r *tokenRepository) StoreRefreshToken(userID string, refreshToken string) error {
	log.Printf("Storing refresh token for user: %s", userID)
	key := fmt.Sprintf("refresh_token:%s:%s", userID, refreshToken)

	// Calculate expiration duration from milliseconds
	expirationDuration := time.Duration(config.Env.JWTRefreshExpirationMilliseconds) * time.Millisecond

	err := r.redis.Set(key, []byte("valid"), expirationDuration, context.Background())
	if err != nil {
		log.Printf("Error storing refresh token: %v", err)
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	log.Printf("Successfully stored refresh token with expiration: %v", expirationDuration)
	return nil
}

func (r *tokenRepository) ValidateRefreshToken(userID string, refreshToken string) bool {
	log.Printf("Validating refresh token for user: %s", userID)
	key := fmt.Sprintf("refresh_token:%s:%s", userID, refreshToken)

	value, err := r.redis.Get(key, context.Background())
	if err != nil {
		log.Printf("Refresh token validation failed: %v", err)
		return false
	}

	log.Printf("Refresh token validated successfully")
	return value == "valid"
}

func (r *tokenRepository) DeleteRefreshToken(userID string, refreshToken string) error {
	log.Printf("Deleting refresh token for user: %s", userID)
	key := fmt.Sprintf("refresh_token:%s:%s", userID, refreshToken)

	// Verify token exists before deletion
	_, err := r.redis.Get(key, context.Background())
	if err != nil {
		log.Printf("Refresh token not found: %v", err)
		return errors.New("refresh token not found")
	}

	err = r.redis.Del(key, context.Background())
	if err != nil {
		log.Printf("Error deleting refresh token: %v", err)
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	log.Printf("Successfully deleted refresh token")
	return nil
}

func (r *tokenRepository) BlacklistToken(token string, expiresAt time.Duration) error {
	log.Printf("Blacklisting token with expiration: %v", expiresAt)
	key := fmt.Sprintf("blacklist:%s", token)

	err := r.redis.Set(key, []byte("blacklisted"), expiresAt, context.Background())
	if err != nil {
		log.Printf("Error blacklisting token: %v", err)
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	log.Printf("Successfully blacklisted token")
	return nil
}

func (r *tokenRepository) IsTokenBlacklisted(token string) bool {
	key := fmt.Sprintf("blacklist:%s", token)
	value, err := r.redis.Get(key, context.Background())
	if err != nil {
		return false
	}
	return value == "blacklisted"
}
