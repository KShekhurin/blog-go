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

const (
	AuthType     = "auth"
	RegisterType = "register"
	IssuerName   = "blog"
)

type TokenClaims struct {
	Type string `json:"type" binding:"required"`
	jwt.RegisteredClaims
}

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
		if errors.Is(err, repositories.ErrorDoesNotExist) {
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
		&TokenClaims{
			Type: AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    IssuerName,
				Subject:   user.ID.String(),
				IssuedAt:  jwt.NewNumericDate(iat),
				ExpiresAt: jwt.NewNumericDate(iat.Add(15 * time.Minute)),
			},
		}).SignedString(service.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	refresh_token, err := jwt.NewWithClaims(
		service.signMethod,
		&TokenClaims{
			Type: RegisterType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    IssuerName,
				Subject:   user.ID.String(),
				IssuedAt:  jwt.NewNumericDate(iat),
				ExpiresAt: jwt.NewNumericDate(iat.Add(30 * 24 * time.Hour)),
			},
		}).SignedString(service.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	return &TokenPair{
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	}, nil
}
