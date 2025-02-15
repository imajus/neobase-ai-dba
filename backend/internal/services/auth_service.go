package services

import (
	"errors"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/utils"
	"net/http"
	"time"
)

type AuthService interface {
	Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error)
	Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error)
	GenerateUserSignupSecret(req *dtos.UserSignupSecretRequest) (*models.UserSignupSecret, uint, error)
}

type authService struct {
	userRepo   repositories.UserRepository
	jwtService utils.JWTService
}

func NewAuthService(userRepo repositories.UserRepository, jwtService utils.JWTService) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtService: jwtService,
	}
}

func (s *authService) Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error) {
	// Check if user exists

	if req.Username == config.Env.AdminUser {
		return nil, http.StatusBadRequest, errors.New("username already exists")
	}

	validUserSignupSecret := s.userRepo.ValidateUserSignupSecret(req.UserSignupSecret)
	if !validUserSignupSecret {
		return nil, http.StatusUnauthorized, errors.New("invalid user signup secret")
	}
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
		Username: req.Username,
		Password: hashedPassword,
		Base: models.Base{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
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

	go func() {
		err := s.userRepo.DeleteUserSignupSecret(req.UserSignupSecret)
		if err != nil {
			log.Println("failed to delete user signup secret:" + err.Error())
		}
	}()

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *user,
	}, http.StatusCreated, nil
}

func (s *authService) Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error) {
	var authUser *models.User
	// Check if it's Admin User
	if req.Username == config.Env.AdminUser {
		if req.Password != config.Env.AdminPassword {
			return nil, http.StatusUnauthorized, errors.New("invalid password")
		}
		user, err := s.userRepo.FindByUsername(req.Username)
		// Checking if Admin user exists in the DB, if not then create user for admin creds
		if err != nil || user == nil {

			// Hash password
			hashedPassword, err := utils.HashPassword(req.Password)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}

			// Create user
			authUser = &models.User{
				Username: req.Username,
				Password: hashedPassword,
				Base: models.Base{
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}

			if err := s.userRepo.Create(authUser); err != nil {
				return nil, http.StatusBadRequest, err
			}
		}
	} else {
		authUser, err := s.userRepo.FindByUsername(req.Username)
		if err != nil {
			return nil, http.StatusNotFound, err
		}
		if authUser == nil {
			return nil, http.StatusUnauthorized, errors.New("invalid credentials")
		}

		if !utils.CheckPasswordHash(req.Password, authUser.Password) {
			return nil, http.StatusUnauthorized, errors.New("invalid credentials")
		}
	}
	accessToken, err := s.jwtService.GenerateToken(authUser.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(authUser.ID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *authUser,
	}, http.StatusOK, nil
}

func (s *authService) GenerateUserSignupSecret(req *dtos.UserSignupSecretRequest) (*models.UserSignupSecret, uint, error) {
	if req.Username != config.Env.AdminUser || req.Password != config.Env.AdminPassword {
		return nil, http.StatusUnauthorized, errors.New("invalid credentials for the admin")
	}

	secret := "UNIQUE_KEY"

	createdSecret, err := s.userRepo.CreateUserSignUpSecret(secret)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return createdSecret, http.StatusCreated, nil
}
