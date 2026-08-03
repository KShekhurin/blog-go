//go:build integration

package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addTokenTestUser(t *testing.T, ctx context.Context, repo UserRepository, login string) *database.User {
	t.Helper()

	user := &database.User{
		ID:           uuid.New(),
		Login:        login,
		Email:        login + "@example.com",
		PasswordHash: "hashed_password",
	}
	require.NoError(t, repo.AddUser(ctx, user))

	return user
}

func countTokensByUserId(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userId uuid.UUID) int64 {
	t.Helper()

	var cnt int64
	err := pool.
		QueryRow(ctx, "SELECT COUNT(*) FROM allowed_refresh_tokens WHERE user_id = $1", userId).
		Scan(&cnt)
	require.NoError(t, err)

	return cnt
}

func TestAddToken(t *testing.T) {
	t.Run("token added", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)
		tokenRepo := NewTokenRepository(q)

		user := addTokenTestUser(t, ctx, userRepo, "token_add_user")

		jti := uuid.New()
		expiresAt := time.Now().Add(time.Hour)

		err := tokenRepo.AddToken(ctx, jti, user.ID, expiresAt)
		require.NoError(t, err)

		// Verify token was actually persisted
		var found database.AllowedRefreshToken
		err = pool.QueryRow(ctx,
			"SELECT jti, user_id, expires_at FROM allowed_refresh_tokens WHERE jti = $1",
			jti,
		).Scan(&found.Jti, &found.UserID, &found.ExpiresAt)
		require.NoError(t, err)
		assert.Equal(t, jti, found.Jti)
		assert.Equal(t, user.ID, found.UserID)
		assert.WithinDuration(t, expiresAt, found.ExpiresAt.Time, time.Second)
	})

	t.Run("token not added due to unique violation, throws error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)
		tokenRepo := NewTokenRepository(q)

		user := addTokenTestUser(t, ctx, userRepo, "token_dup_user")

		jti := uuid.New()
		expiresAt := time.Now().Add(time.Hour)

		// First insert should succeed
		err := tokenRepo.AddToken(ctx, jti, user.ID, expiresAt)
		require.NoError(t, err)

		// Second insert with the same jti should fail
		err = tokenRepo.AddToken(ctx, jti, user.ID, expiresAt)
		assert.ErrorIs(t, err, errs.ErrAlreadyExists)
	})
}

func TestTryToDeleteToken(t *testing.T) {
	t.Run("successfully deletes token, returns true", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)
		tokenRepo := NewTokenRepository(q)

		user := addTokenTestUser(t, ctx, userRepo, "token_del_user")

		jti := uuid.New()
		require.NoError(t, tokenRepo.AddToken(ctx, jti, user.ID, time.Now().Add(time.Hour)))

		isSuccess, err := tokenRepo.TryToDeleteToken(ctx, jti)
		require.NoError(t, err)
		assert.True(t, isSuccess)

		// Verify token was actually removed
		var cnt int64
		err = pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM allowed_refresh_tokens WHERE jti = $1",
			jti,
		).Scan(&cnt)
		require.NoError(t, err)
		assert.Equal(t, int64(0), cnt)
	})

	t.Run("does not delete one, returns false", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		tokenRepo := NewTokenRepository(q)

		isSuccess, err := tokenRepo.TryToDeleteToken(ctx, uuid.New())
		require.NoError(t, err)
		assert.False(t, isSuccess)
	})
}

func TestDeleteAllUserTokens(t *testing.T) {
	t.Run("deletes all user tokens", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)
		tokenRepo := NewTokenRepository(q)

		user := addTokenTestUser(t, ctx, userRepo, "token_delall_user")
		otherUser := addTokenTestUser(t, ctx, userRepo, "token_delall_other")

		expiresAt := time.Now().Add(time.Hour)
		for range 3 {
			require.NoError(t, tokenRepo.AddToken(ctx, uuid.New(), user.ID, expiresAt))
		}
		require.NoError(t, tokenRepo.AddToken(ctx, uuid.New(), otherUser.ID, expiresAt))

		err := tokenRepo.DeleteAllUserTokens(ctx, user.ID)
		require.NoError(t, err)

		// Verify all tokens of the user were removed
		assert.Equal(t, int64(0), countTokensByUserId(t, ctx, pool, user.ID))

		// Verify other user's tokens are untouched
		assert.Equal(t, int64(1), countTokensByUserId(t, ctx, pool, otherUser.ID))
	})
}
