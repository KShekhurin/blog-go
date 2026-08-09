//go:build integration

package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthService(ctx context.Context, t testing.TB) (AuthService, repositories.UserRepository, ed25519.PublicKey, *pgxpool.Pool) {
	t.Helper()

	pool := migrations.SetupPostgres(ctx, t)
	queries := database.New(pool)

	userRepo := repositories.NewUserRepository(queries)
	tokenRepo := repositories.NewTokenRepository(queries)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	service := NewAuthService(userRepo, tokenRepo, privateKey, jwt.SigningMethodEdDSA)

	return service, userRepo, publicKey, pool
}

func addUserWithPassword(ctx context.Context, t testing.TB, repo repositories.UserRepository, login, email, password string) *database.User {
	t.Helper()

	hash, err := argon2id.CreateHash(password, testHashParams)
	require.NoError(t, err)

	user := &database.User{
		ID:           uuid.New(),
		Login:        login,
		Email:        email,
		PasswordHash: hash,
	}
	require.NoError(t, repo.AddUser(ctx, user))

	return user
}

func parseTokenClaims(t *testing.T, tokenString string, publicKey ed25519.PublicKey) *TokenClaims {
	t.Helper()

	claims := &TokenClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return publicKey, nil
	})
	require.NoError(t, err)

	return claims
}

func refreshTokenExists(ctx context.Context, t testing.TB, pool *pgxpool.Pool, jti uuid.UUID) bool {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM allowed_refresh_tokens WHERE jti = $1)", jti).Scan(&exists)
	require.NoError(t, err)

	return exists
}

func TestAuthenticateUser(t *testing.T) {
	const password = "secure_password"

	t.Run("login and email blank returns ErrBadPayload", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, _, _, _ := setupAuthService(ctx, t)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Password: password,
		})

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrBadPayload)
	})

	t.Run("found user by login, valid password, user is returned", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, _, _ := setupAuthService(ctx, t)

		created := addUserWithPassword(ctx, t, userRepo, "auth_login_user", "auth_login_user@example.com", password)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "auth_login_user",
			Password: password,
		})

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, created.ID, user.ID)
		assert.Equal(t, created.Login, user.Login)
		assert.Equal(t, created.Email, user.Email)
	})

	t.Run("found user by email, valid password, user is returned", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, _, _ := setupAuthService(ctx, t)

		created := addUserWithPassword(ctx, t, userRepo, "auth_email_user", "auth_email_user@example.com", password)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Email:    "auth_email_user@example.com",
			Password: password,
		})

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, created.ID, user.ID)
		assert.Equal(t, created.Login, user.Login)
		assert.Equal(t, created.Email, user.Email)
	})

	t.Run("invalid password returns ErrInvalidCredentials", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, _, _ := setupAuthService(ctx, t)

		addUserWithPassword(ctx, t, userRepo, "auth_wrong_pass", "auth_wrong_pass@example.com", password)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "auth_wrong_pass",
			Password: "definitely_wrong_password",
		})

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("user not found returns ErrInvalidCredentials", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, _, _, _ := setupAuthService(ctx, t)

		user, err := service.AuthenticateUser(ctx, &webModels.UserLoginInfo{
			Login:    "no_such_user",
			Email:    "no_such_user@example.com",
			Password: password,
		})

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})
}

func TestSignJWT(t *testing.T) {
	t.Run("signed token pair, refresh jti is persisted in database", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, publicKey, pool := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "sign_user", "sign_user@example.com", "secure_password")

		pair, err := service.SignJWT(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, pair)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)

		accessClaims := parseTokenClaims(t, pair.AccessToken, publicKey)
		assert.Equal(t, AuthType, accessClaims.Type)
		assert.Equal(t, IssuerName, accessClaims.Issuer)
		assert.Equal(t, user.ID.String(), accessClaims.Subject)

		refreshClaims := parseTokenClaims(t, pair.RefreshToken, publicKey)
		assert.Equal(t, RefreshType, refreshClaims.Type)
		assert.Equal(t, IssuerName, refreshClaims.Issuer)
		assert.Equal(t, user.ID.String(), refreshClaims.Subject)

		jti, err := uuid.Parse(refreshClaims.ID)
		require.NoError(t, err)
		assert.True(t, refreshTokenExists(ctx, t, pool, jti),
			"refresh token jti should be stored in the database")
	})
}

func TestRotateJWT(t *testing.T) {
	t.Run("token is deleted and new pair is issued", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, publicKey, pool := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "rotate_user", "rotate_user@example.com", "secure_password")

		pair, err := service.SignJWT(ctx, user.ID)
		require.NoError(t, err)

		oldJti, err := uuid.Parse(parseTokenClaims(t, pair.RefreshToken, publicKey).ID)
		require.NoError(t, err)

		newPair, err := service.RotateJWT(ctx, oldJti, user.ID)
		require.NoError(t, err)
		require.NotNil(t, newPair)
		assert.NotEmpty(t, newPair.AccessToken)
		assert.NotEmpty(t, newPair.RefreshToken)
		assert.NotEqual(t, pair.RefreshToken, newPair.RefreshToken)

		// old token is gone, new token is stored
		assert.False(t, refreshTokenExists(ctx, t, pool, oldJti))

		newJti, err := uuid.Parse(parseTokenClaims(t, newPair.RefreshToken, publicKey).ID)
		require.NoError(t, err)
		assert.True(t, refreshTokenExists(ctx, t, pool, newJti))

		// rotating with the old jti again must fail
		_, err = service.RotateJWT(ctx, oldJti, user.ID)
		assert.ErrorIs(t, err, ErrTokenBanned)
	})

	t.Run("token not in db returns ErrTokenBanned", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, _, _ := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "rotate_banned", "rotate_banned@example.com", "secure_password")

		pair, err := service.RotateJWT(ctx, uuid.New(), user.ID)
		assert.Nil(t, pair)
		assert.ErrorIs(t, err, ErrTokenBanned)
	})

	t.Run("concurrent rotation: only one goroutine succeeds", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, publicKey, _ := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "rotate_race", "rotate_race@example.com", "secure_password")

		pair, err := service.SignJWT(ctx, user.ID)
		require.NoError(t, err)

		jti, err := uuid.Parse(parseTokenClaims(t, pair.RefreshToken, publicKey).ID)
		require.NoError(t, err)

		const workers = 10
		errsCh := make(chan error, workers)
		var wg sync.WaitGroup

		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := service.RotateJWT(ctx, jti, user.ID)
				errsCh <- err
			}()
		}

		wg.Wait()
		close(errsCh)

		var successCount, bannedCount int
		for err := range errsCh {
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, ErrTokenBanned):
				bannedCount++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}

		assert.Equal(t, 1, successCount, "exactly one goroutine should rotate successfully")
		assert.Equal(t, workers-1, bannedCount, "all other goroutines should get ErrTokenBanned")
	})
}

func TestLogoutByRefresh(t *testing.T) {
	t.Run("successfully logged out", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, publicKey, pool := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "logout_user", "logout_user@example.com", "secure_password")

		pair, err := service.SignJWT(ctx, user.ID)
		require.NoError(t, err)

		jti, err := uuid.Parse(parseTokenClaims(t, pair.RefreshToken, publicKey).ID)
		require.NoError(t, err)

		require.NoError(t, service.LogoutByRefresh(ctx, jti))
		assert.False(t, refreshTokenExists(ctx, t, pool, jti))
	})

	t.Run("cannot log out twice", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo, publicKey, _ := setupAuthService(ctx, t)

		user := addUserWithPassword(ctx, t, userRepo, "logout_twice", "logout_twice@example.com", "secure_password")

		pair, err := service.SignJWT(ctx, user.ID)
		require.NoError(t, err)

		jti, err := uuid.Parse(parseTokenClaims(t, pair.RefreshToken, publicKey).ID)
		require.NoError(t, err)

		require.NoError(t, service.LogoutByRefresh(ctx, jti))

		err = service.LogoutByRefresh(ctx, jti)
		assert.ErrorIs(t, err, ErrTokenBanned)
	})

	t.Run("was not logged in in the first place returns ErrTokenBanned", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, _, _, _ := setupAuthService(ctx, t)

		err := service.LogoutByRefresh(ctx, uuid.New())
		assert.ErrorIs(t, err, ErrTokenBanned)
	})
}
