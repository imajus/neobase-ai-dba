package services

import (
	"errors"
	"neobase-ai/internal/dtos"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repository"
	"neobase-ai/internal/utils"
	"net/http"
	"time"
)

type AuthService interface {
	Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error)
	Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error)
}

type authService struct {
	userRepo   repository.UserRepository
	jwtService utils.JWTService
}

func NewAuthService(userRepo repository.UserRepository, jwtService utils.JWTService) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtService: jwtService,
	}
}

func (s *authService) Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error) {
	// Check if user exists
	existingUser, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, http.StatusNotFound, err
	}
	if existingUser != nil {
		return nil, http.StatusBadRequest, errors.New("username already exists")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Create user
	user := &models.User{
		Username:  req.Username,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Generate token
	accessToken, err := s.jwtService.GenerateToken(user.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *user,
	}, http.StatusCreated, nil
}

func (s *authService) Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, http.StatusNotFound, err
	}
	if user == nil {
		return nil, http.StatusUnauthorized, errors.New("invalid credentials")
	}

	if !utils.CheckPasswordHash(req.Password, user.Password) {
		return nil, http.StatusUnauthorized, errors.New("invalid credentials")
	}

	accessToken, err := s.jwtService.GenerateToken(user.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *user,
	}, http.StatusOK, nil
}
