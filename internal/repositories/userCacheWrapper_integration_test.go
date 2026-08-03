//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCacheWrapper_SubscribeUserTo(t *testing.T) {
	t.Run("no sub in cache, added to cache & postgres, no err", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		// Setup users
		author := &database.User{
			ID:           uuid.New(),
			Login:        "author1",
			Email:        "author1@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub1",
			Email:        "sub1@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		// Test subscription
		err := wrapper.SubscribeUserTo(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify postgres
		pgSubs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Contains(t, pgSubs, sub.ID)

		// Verify cache
		cacheSubs, err := subsCache.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Contains(t, cacheSubs, sub.ID)
	})

	t.Run("sub in cache & in postgres, error unique violation", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		// Setup users
		author := &database.User{
			ID:           uuid.New(),
			Login:        "author2",
			Email:        "author2@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub2",
			Email:        "sub2@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		// Pre-populate both postgres and cache
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub.ID, author.ID))
		require.NoError(t, subsCache.SubscribeUserTo(ctx, sub.ID, author.ID))

		// Attempt duplicate subscription should fail with unique violation
		err := wrapper.SubscribeUserTo(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, errs.ErrAlreadyExists)
	})
}

func TestUserCacheWrapper_UnsubscribeUserFrom(t *testing.T) {
	t.Run("is in cache & postgres, successfully removed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		// Setup users
		author := &database.User{
			ID:           uuid.New(),
			Login:        "author3",
			Email:        "author3@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub3",
			Email:        "sub3@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		// Pre-populate both postgres and cache
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub.ID, author.ID))
		require.NoError(t, subsCache.SubscribeUserTo(ctx, sub.ID, author.ID))

		// Test unsubscription
		err := wrapper.UnsubscribeUserFrom(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify postgres removal
		pgSubs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.NotContains(t, pgSubs, sub.ID)

		// Verify cache removal (note: redis set is empty, so GetSubs returns ErrorDoesNotExist)
		_, err = subsCache.GetSubs(ctx, author.ID)
		assert.ErrorIs(t, err, errs.ErrNotFound)
	})

	t.Run("not in repo & cache returns not exists error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		// Setup users
		author := &database.User{
			ID:           uuid.New(),
			Login:        "author4",
			Email:        "author4@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub4",
			Email:        "sub4@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		// Attempt to unsubscribe non-existent subscription
		err := wrapper.UnsubscribeUserFrom(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, errs.ErrNotFound)
	})
}
