package dtos

import "neobase-ai/internal/models"

type SignupRequest struct {
	Username         string `json:"username" binding:"required"`
	Password         string `json:"password" binding:"required,min=6"`
	UserSignupSecret string `json:"user_signup_secret"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserSignupSecretRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         models.User `json:"user"`
}
