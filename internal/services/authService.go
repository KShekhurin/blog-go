package services

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrorInvalidCredentials = errors.New("invalid credentials")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService interface {
	AuthenticateUser(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error)
	SignJWT(ctx context.Context, userInfo *database.User) (*TokenPair, error)
}

type authService struct {
	userRepo   repositories.UserRepository
	privateKey ed25519.PrivateKey
	signMethod jwt.SigningMethod
}

func NewAuthService(userRepo repositories.UserRepository, privateKey ed25519.PrivateKey, signMethod jwt.SigningMethod) AuthService {
	return &authService{
		userRepo:   userRepo,
		privateKey: privateKey,
		signMethod: signMethod,
	}
}

func (service *authService) AuthenticateUser(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
	if userInfo.Login == "" && userInfo.Email == "" {
		return nil, ErrorBadPayload
	}

	user, err := service.userRepo.FindUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrorUserDoesNotExist) {
			return nil, ErrorInvalidCredentials
		}
		return nil, fmt.Errorf("find user by login or email failed: %w", err)
	}

	match, err := argon2id.ComparePasswordAndHash(userInfo.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("compare password failed: %w", err)
	}
	if !match {
		return nil, ErrorInvalidCredentials
	}

	return user, nil
}

func (service *authService) SignJWT(ctx context.Context, user *database.User) (*TokenPair, error) {
	iat := time.Now()

	access_token, err := jwt.NewWithClaims(
		service.signMethod,
		jwt.MapClaims{
			"iat": iat.Unix(),
			"exp": iat.Add(15 * time.Minute).Unix(),
			"sub": user.ID,
			"iss": "blog",
		}).SignedString(service.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	refresh_token, err := jwt.NewWithClaims(
		service.signMethod,
		jwt.MapClaims{
			"iat":        iat.Unix(),
			"exp":        iat.Add(24 * time.Hour * 30).Unix(),
			"sub":        user.ID,
			"iss":        "blog",
			"is_refresh": true,
		}).SignedString(service.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	return &TokenPair{
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	}, nil
}
