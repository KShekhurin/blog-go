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
	"github.com/google/uuid"
)

var (
	ErrorInvalidCredentials = errors.New("invalid credentials")
)

const (
	AuthType    = "auth"
	RefreshType = "refresh"
	IssuerName  = "blog"
)

type TokenClaims struct {
	Type string `json:"type" binding:"required"`
	jwt.RegisteredClaims
}

type AuthService interface {
	AuthenticateUser(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error)
	SignJWT(ctx context.Context, userId uuid.UUID) (*webModels.TokenPair, error)
	RotateJWT(ctx context.Context, jti uuid.UUID, userId uuid.UUID) (*webModels.TokenPair, error)
	LogoutByRefresh(ctx context.Context, jti uuid.UUID) error
}

type authService struct {
	userRepo   repositories.UserRepository
	tokenRepo  repositories.TokenRepository
	privateKey ed25519.PrivateKey
	signMethod jwt.SigningMethod
}

func NewAuthService(userRepo repositories.UserRepository, tokenRepo repositories.TokenRepository, privateKey ed25519.PrivateKey, signMethod jwt.SigningMethod) AuthService {
	return &authService{
		userRepo:   userRepo,
		privateKey: privateKey,
		tokenRepo:  tokenRepo,
		signMethod: signMethod,
	}
}

func (s *authService) AuthenticateUser(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
	if userInfo.Login == "" && userInfo.Email == "" {
		return nil, ErrorBadPayload
	}

	user, err := s.userRepo.FindUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)
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

func (s *authService) SignJWT(ctx context.Context, userId uuid.UUID) (*webModels.TokenPair, error) {
	iat := time.Now()

	accessToken, err := jwt.NewWithClaims(
		s.signMethod,
		&TokenClaims{
			Type: AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    IssuerName,
				Subject:   userId.String(),
				IssuedAt:  jwt.NewNumericDate(iat),
				ExpiresAt: jwt.NewNumericDate(iat.Add(15 * time.Minute)),
			},
		}).SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	jti := uuid.New()
	refreshExpiration := iat.Add(30 * 24 * time.Hour)

	refreshToken, err := jwt.NewWithClaims(
		s.signMethod,
		&TokenClaims{
			Type: RefreshType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    IssuerName,
				Subject:   userId.String(),
				IssuedAt:  jwt.NewNumericDate(iat),
				ExpiresAt: jwt.NewNumericDate(refreshExpiration),
				ID:        jti.String(),
			},
		}).SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign jwt failed: %w", err)
	}

	err = s.tokenRepo.AddToken(ctx, jti, userId, refreshExpiration)
	if err != nil {
		return nil, fmt.Errorf("add token failed: %w", err)
	}

	return &webModels.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *authService) RotateJWT(ctx context.Context, jti uuid.UUID, userId uuid.UUID) (*webModels.TokenPair, error) {
	isSuccess, err := s.tokenRepo.TryToDeleteToken(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("try to delete token failed: %w", err)
	}

	if !isSuccess {
		return nil, ErrorInvalidCredentials
	}

	return s.SignJWT(ctx, userId)
}

func (s *authService) LogoutByRefresh(ctx context.Context, jti uuid.UUID) error {
	isSuccess, err := s.tokenRepo.TryToDeleteToken(ctx, jti)

	if err != nil {
		return fmt.Errorf("try to delete token failed: %w", err)
	}

	if !isSuccess {
		return ErrorInvalidCredentials
	}

	return nil
}
