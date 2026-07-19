package services

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrorInvalidCredentials = errors.New("invalid credentials")
)

type AuthService interface {
	SignJWT(ctx context.Context, userInfo *webModels.UserLoginInfo) (string, error)
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

func (service *authService) SignJWT(ctx context.Context, userInfo *webModels.UserLoginInfo) (string, error) {
	if userInfo.Login == "" && userInfo.Email == "" {
		return "", ErrorBadPayload
	}

	user, err := service.userRepo.FindUserByLoginOrEmail(ctx, userInfo.Email, userInfo.Login)
	if err != nil {
		if errors.Is(err, repositories.ErrorUserDoesNotExist) {
			return "", ErrorInvalidCredentials
		}
		return "", fmt.Errorf("find user by login or email failed: %w", err)
	}

	match, err := argon2id.ComparePasswordAndHash(userInfo.Password, user.PasswordHash)
	if err != nil {
		return "", fmt.Errorf("compare password failed: %w", err)
	}
	if !match {
		return "", ErrorInvalidCredentials
	}

	token := jwt.NewWithClaims(
		service.signMethod,
		jwt.MapClaims{
			"iss":     "blog-go",
			"user_id": user.ID,
		})

	return token.SignedString(service.privateKey)
}
