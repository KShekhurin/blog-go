package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stub ---

type authUserRepositoryStub struct {
	findUser *database.User
	findErr  error

	findCalls int
}

var _ repositories.UserRepository = (*authUserRepositoryStub)(nil)

func (s *authUserRepositoryStub) FindUserById(ctx context.Context, id uuid.UUID) (*database.User, error) {
	return nil, nil
}

func (s *authUserRepositoryStub) FindUserByLogin(ctx context.Context, login string) (*database.User, error) {
	return nil, nil
}

func (s *authUserRepositoryStub) FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	s.findCalls++
	return s.findUser, s.findErr
}

func (s *authUserRepositoryStub) AddUser(ctx context.Context, user *database.User) error {
	return nil
}

func (s *authUserRepositoryStub) SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	return nil
}

func (s *authUserRepositoryStub) UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	return nil
}

// --- helpers ---

func generateTestKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return privateKey
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	require.NoError(t, err)
	return hash
}

// --- AuthenticateUser tests ---

const testPassword = "password"

func TestAuthenticateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("returns ErrorBadPayload when both login and email are empty", func(t *testing.T) {
		stub := &authUserRepositoryStub{}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "",
			Email:    "",
			Password: testPassword,
		})

		assert.ErrorIs(t, err, ErrorBadPayload)
		assert.Nil(t, user)
		assert.Equal(t, 0, stub.findCalls, "repository must not be called on bad payload")
	})

	t.Run("returns ErrorInvalidCredentials when user does not exist", func(t *testing.T) {
		stub := &authUserRepositoryStub{
			findErr: repositories.ErrorUserDoesNotExist,
		}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "tom",
			Password: testPassword,
		})

		assert.ErrorIs(t, err, ErrorInvalidCredentials)
		assert.Nil(t, user)
	})

	t.Run("returns wrapped error when repository fails unexpectedly", func(t *testing.T) {
		repoErr := errors.New("connection refused")
		stub := &authUserRepositoryStub{
			findErr: repoErr,
		}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "tom",
			Password: testPassword,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
		assert.ErrorContains(t, err, "find user by login or email failed")
		assert.NotErrorIs(t, err, ErrorInvalidCredentials)
		assert.Nil(t, user)
	})

	t.Run("returns wrapped error when stored hash is malformed", func(t *testing.T) {
		stub := &authUserRepositoryStub{
			findUser: &database.User{
				ID:           uuid.New(),
				Login:        "tom",
				PasswordHash: "not-a-valid-argon2id-hash",
			},
		}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "tom",
			Password: testPassword,
		})

		require.Error(t, err)
		assert.ErrorContains(t, err, "compare password failed")
		assert.NotErrorIs(t, err, ErrorInvalidCredentials)
		assert.Nil(t, user)
	})

	t.Run("returns ErrorInvalidCredentials when password does not match", func(t *testing.T) {
		stub := &authUserRepositoryStub{
			findUser: &database.User{
				ID:           uuid.New(),
				Login:        "tom",
				PasswordHash: mustHash(t, "other-password"),
			},
		}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "tom",
			Password: testPassword,
		})

		assert.ErrorIs(t, err, ErrorInvalidCredentials)
		assert.Nil(t, user)
	})

	t.Run("authenticates by login", func(t *testing.T) {
		expectedUser := &database.User{
			ID:           uuid.New(),
			Login:        "tom",
			Email:        "tom@gmail.com",
			PasswordHash: mustHash(t, testPassword),
		}
		stub := &authUserRepositoryStub{findUser: expectedUser}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "tom",
			Password: testPassword,
		})

		require.NoError(t, err)
		assert.Same(t, expectedUser, user)
		assert.Equal(t, 1, stub.findCalls)
	})

	t.Run("authenticates by email", func(t *testing.T) {
		expectedUser := &database.User{
			ID:           uuid.New(),
			Login:        "tom",
			Email:        "tom@gmail.com",
			PasswordHash: mustHash(t, testPassword),
		}
		stub := &authUserRepositoryStub{findUser: expectedUser}
		service := NewAuthService(stub, generateTestKey(t), jwt.SigningMethodEdDSA)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Email:    "tom@gmail.com",
			Password: testPassword,
		})

		require.NoError(t, err)
		assert.Same(t, expectedUser, user)
		assert.Equal(t, 1, stub.findCalls)
	})
}

// --- SignJWT tests ---

func TestSignJWT(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when signing method is incompatible with the key", func(t *testing.T) {
		privateKey := generateTestKey(t)
		service := NewAuthService(&authUserRepositoryStub{}, privateKey, jwt.SigningMethodHS256)

		tokenPair, err := service.SignJWT(ctx, &database.User{ID: uuid.New()})

		require.Error(t, err)
		assert.ErrorContains(t, err, "sign jwt failed")
		assert.Nil(t, tokenPair)
	})

	t.Run("issues verifiable access and refresh tokens with expected claims and TTLs", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			privateKey := generateTestKey(t)
			service := NewAuthService(&authUserRepositoryStub{}, privateKey, jwt.SigningMethodEdDSA)
			user := &database.User{ID: uuid.New(), Login: "tom", Email: "tom@gmail.com"}

			start := time.Now()

			tokenPair, err := service.SignJWT(ctx, user)
			require.NoError(t, err)
			require.NotNil(t, tokenPair)
			require.NotEmpty(t, tokenPair.AccessToken)
			require.NotEmpty(t, tokenPair.RefreshToken)
			assert.NotEqual(t, tokenPair.AccessToken, tokenPair.RefreshToken)

			parse := func(tokenString string) jwt.MapClaims {
				token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
					assert.Equal(t, jwt.SigningMethodEdDSA.Alg(), token.Method.Alg())
					return privateKey.Public(), nil
				})
				require.NoError(t, err)
				require.True(t, token.Valid)
				claims, ok := token.Claims.(jwt.MapClaims)
				require.True(t, ok)
				return claims
			}

			accessClaims := parse(tokenPair.AccessToken)
			assert.Equal(t, user.ID.String(), accessClaims["sub"])
			assert.Equal(t, "blog", accessClaims["iss"])
			assert.Equal(t, float64(start.Unix()), accessClaims["iat"])
			assert.Equal(t, float64(start.Add(15*time.Minute).Unix()), accessClaims["exp"])
			assert.Nil(t, accessClaims["is_refresh"], "access token must not be marked as refresh")

			refreshClaims := parse(tokenPair.RefreshToken)
			assert.Equal(t, user.ID.String(), refreshClaims["sub"])
			assert.Equal(t, "blog", refreshClaims["iss"])
			assert.Equal(t, float64(start.Unix()), refreshClaims["iat"])
			assert.Equal(t, float64(start.Add(24*time.Hour*30).Unix()), refreshClaims["exp"])
			assert.Equal(t, true, refreshClaims["is_refresh"])
		})
	})

	t.Run("access token expires 15 minutes after issue", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			privateKey := generateTestKey(t)
			service := NewAuthService(&authUserRepositoryStub{}, privateKey, jwt.SigningMethodEdDSA)
			user := &database.User{ID: uuid.New()}

			tokenPair, err := service.SignJWT(ctx, user)
			require.NoError(t, err)

			parse := func(tokenString string) (*jwt.Token, error) {
				return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
					return privateKey.Public(), nil
				})
			}

			// still valid right before expiration
			time.Sleep(15*time.Minute - time.Second)
			token, err := parse(tokenPair.AccessToken)
			require.NoError(t, err)
			assert.True(t, token.Valid)

			// expired one second past the TTL
			time.Sleep(2 * time.Second)
			_, err = parse(tokenPair.AccessToken)
			require.Error(t, err)
			assert.ErrorIs(t, err, jwt.ErrTokenExpired)

			// refresh token is still valid after access token expired
			refreshToken, err := parse(tokenPair.RefreshToken)
			require.NoError(t, err)
			assert.True(t, refreshToken.Valid)
		})
	})
}
