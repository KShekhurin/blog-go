//go:build integration

package repositories

import (
	"context"
	"fmt"
	"sync"
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
		// Follower that exists only in the repo and should be picked up
		// by the cache refresh triggered during SubscribeUserTo
		repoOnlySub := &database.User{
			ID:           uuid.New(),
			Login:        "repo_only_sub1",
			Email:        "repo_only_sub1@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))
		require.NoError(t, userRepo.AddUser(ctx, repoOnlySub))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, repoOnlySub.ID, author.ID))

		// Test subscription
		err := wrapper.SubscribeUserTo(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify postgres
		pgSubs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub.ID, repoOnlySub.ID}, pgSubs)

		// Verify the cache was refreshed and now holds every follower,
		// including the one that was not in the cache before
		cacheSubs, err := subsCache.GetFollows(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub.ID, repoOnlySub.ID}, cacheSubs)
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

	t.Run("no subs in cache, concurrent subscriptions all succeed and appear in cache", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		const goroutines = 10

		// Each goroutine subscribes a distinct subscriber to its own author,
		// so no two goroutines race on the same cache key
		authors := make([]uuid.UUID, goroutines)
		subs := make([]uuid.UUID, goroutines)
		for i := range goroutines {
			author := &database.User{
				ID:           uuid.New(),
				Login:        fmt.Sprintf("cs_author_%d", i),
				Email:        fmt.Sprintf("cs_author_%d@example.com", i),
				PasswordHash: "hash",
			}
			sub := &database.User{
				ID:           uuid.New(),
				Login:        fmt.Sprintf("cs_sub_%d", i),
				Email:        fmt.Sprintf("cs_sub_%d@example.com", i),
				PasswordHash: "hash",
			}
			require.NoError(t, userRepo.AddUser(ctx, author))
			require.NoError(t, userRepo.AddUser(ctx, sub))
			authors[i] = author.ID
			subs[i] = sub.ID
		}

		start := make(chan struct{})
		subErrs := make([]error, goroutines)
		var wg sync.WaitGroup
		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				subErrs[i] = wrapper.SubscribeUserTo(ctx, subs[i], authors[i])
			}()
		}
		close(start)
		wg.Wait()

		for i := range goroutines {
			require.NoError(t, subErrs[i])

			pgSubs, err := userRepo.GetSubs(ctx, authors[i])
			require.NoError(t, err)
			assert.Contains(t, pgSubs, subs[i])

			cacheSubs, err := subsCache.GetFollows(ctx, authors[i])
			require.NoError(t, err)
			assert.Contains(t, cacheSubs, subs[i])
		}
	})

	t.Run("no followers in cache & repo, first subscription succeeds and is cached", func(t *testing.T) {
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
			Login:        "author6",
			Email:        "author6@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub6",
			Email:        "sub6@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		// Sanity check: the author has no followers anywhere yet
		pgSubs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		require.Empty(t, pgSubs)
		_, err = subsCache.GetFollows(ctx, author.ID)
		require.ErrorIs(t, err, errs.ErrNotFound)

		// Test subscription
		err = wrapper.SubscribeUserTo(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify postgres
		pgSubs, err = userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub.ID}, pgSubs)

		// Verify the refresh populated the cache with the first follower
		cacheSubs, err := subsCache.GetFollows(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub.ID}, cacheSubs)
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

		// Verify cache removal (note: redis set is empty, so GetFollows returns ErrorDoesNotExist)
		_, err = subsCache.GetFollows(ctx, author.ID)
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

		// The repo error short-circuits before any cache interaction,
		// so the cache remains empty
		_, err = subsCache.GetFollows(ctx, author.ID)
		assert.ErrorIs(t, err, errs.ErrNotFound)
	})

	t.Run("not in cache, removed from postgres, remaining followers are cached", func(t *testing.T) {
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
			Login:        "author5",
			Email:        "author5@example.com",
			PasswordHash: "hash",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub5",
			Email:        "sub5@example.com",
			PasswordHash: "hash",
		}
		otherSub := &database.User{
			ID:           uuid.New(),
			Login:        "sub5b",
			Email:        "sub5b@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))
		require.NoError(t, userRepo.AddUser(ctx, otherSub))

		// Populate only the repo, the cache stays empty
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub.ID, author.ID))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, otherSub.ID, author.ID))

		// Test unsubscription
		err := wrapper.UnsubscribeUserFrom(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify postgres removal
		pgSubs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{otherSub.ID}, pgSubs)

		// Verify the cache was refreshed with the remaining followers
		cacheSubs, err := subsCache.GetFollows(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{otherSub.ID}, cacheSubs)
	})

	t.Run("not in cache, concurrent unsubscriptions all succeed and refresh the cache", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		const goroutines = 10

		// Each goroutine works with its own author, so no two goroutines
		// race on the same cache key
		authors := make([]uuid.UUID, goroutines)
		removed := make([]uuid.UUID, goroutines)
		remaining := make([]uuid.UUID, goroutines)
		for i := range goroutines {
			author := &database.User{
				ID:           uuid.New(),
				Login:        fmt.Sprintf("cu_author_%d", i),
				Email:        fmt.Sprintf("cu_author_%d@example.com", i),
				PasswordHash: "hash",
			}
			removedSub := &database.User{
				ID:           uuid.New(),
				Login:        fmt.Sprintf("cu_removed_%d", i),
				Email:        fmt.Sprintf("cu_removed_%d@example.com", i),
				PasswordHash: "hash",
			}
			remainingSub := &database.User{
				ID:           uuid.New(),
				Login:        fmt.Sprintf("cu_keep_%d", i),
				Email:        fmt.Sprintf("cu_keep_%d@example.com", i),
				PasswordHash: "hash",
			}
			require.NoError(t, userRepo.AddUser(ctx, author))
			require.NoError(t, userRepo.AddUser(ctx, removedSub))
			require.NoError(t, userRepo.AddUser(ctx, remainingSub))
			// Populate only the repo, the cache stays empty
			require.NoError(t, userRepo.SubscribeUserTo(ctx, removedSub.ID, author.ID))
			require.NoError(t, userRepo.SubscribeUserTo(ctx, remainingSub.ID, author.ID))
			authors[i] = author.ID
			removed[i] = removedSub.ID
			remaining[i] = remainingSub.ID
		}

		start := make(chan struct{})
		unsubErrs := make([]error, goroutines)
		var wg sync.WaitGroup
		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				unsubErrs[i] = wrapper.UnsubscribeUserFrom(ctx, removed[i], authors[i])
			}()
		}
		close(start)
		wg.Wait()

		for i := range goroutines {
			require.NoError(t, unsubErrs[i])

			pgSubs, err := userRepo.GetSubs(ctx, authors[i])
			require.NoError(t, err)
			assert.ElementsMatch(t, []uuid.UUID{remaining[i]}, pgSubs)

			cacheSubs, err := subsCache.GetFollows(ctx, authors[i])
			require.NoError(t, err)
			assert.ElementsMatch(t, []uuid.UUID{remaining[i]}, cacheSubs)
		}
	})
}

func TestUserCacheWrapper_GetSubs(t *testing.T) {
	t.Run("followers are in cache & repo, returns them", func(t *testing.T) {
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
			Login:        "getsubs_author1",
			Email:        "getsubs_author1@example.com",
			PasswordHash: "hash",
		}
		sub1 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub1",
			Email:        "getsubs_sub1@example.com",
			PasswordHash: "hash",
		}
		sub2 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub2",
			Email:        "getsubs_sub2@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub1))
		require.NoError(t, userRepo.AddUser(ctx, sub2))

		// Followers are present both in the repo and in the cache
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub1.ID, author.ID))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub2.ID, author.ID))
		require.NoError(t, subsCache.SetFollows(ctx, author.ID, []uuid.UUID{sub1.ID, sub2.ID}))

		subs, err := wrapper.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub1.ID, sub2.ID}, subs)
	})

	t.Run("followers not in cache but in repo, fetches them and updates the cache", func(t *testing.T) {
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
			Login:        "getsubs_author2",
			Email:        "getsubs_author2@example.com",
			PasswordHash: "hash",
		}
		sub1 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub3",
			Email:        "getsubs_sub3@example.com",
			PasswordHash: "hash",
		}
		sub2 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub4",
			Email:        "getsubs_sub4@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub1))
		require.NoError(t, userRepo.AddUser(ctx, sub2))

		// Followers are only in the repo, the cache is empty
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub1.ID, author.ID))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub2.ID, author.ID))

		subs, err := wrapper.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub1.ID, sub2.ID}, subs)

		// Verify the followers are now stored in the cache
		cacheSubs, err := subsCache.GetFollows(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub1.ID, sub2.ID}, cacheSubs)
	})

	t.Run("followers not in cache, concurrent GetSubs all succeed", func(t *testing.T) {
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
			Login:        "getsubs_author3",
			Email:        "getsubs_author3@example.com",
			PasswordHash: "hash",
		}
		sub1 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub5",
			Email:        "getsubs_sub5@example.com",
			PasswordHash: "hash",
		}
		sub2 := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_sub6",
			Email:        "getsubs_sub6@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub1))
		require.NoError(t, userRepo.AddUser(ctx, sub2))

		// Followers are only in the repo, the cache is empty
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub1.ID, author.ID))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub2.ID, author.ID))
		want := []uuid.UUID{sub1.ID, sub2.ID}

		const goroutines = 20
		start := make(chan struct{})
		results := make([][]uuid.UUID, goroutines)
		getErrs := make([]error, goroutines)
		var wg sync.WaitGroup
		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results[i], getErrs[i] = wrapper.GetSubs(ctx, author.ID)
			}()
		}
		close(start)
		wg.Wait()

		for i := range goroutines {
			require.NoError(t, getErrs[i])
			assert.ElementsMatch(t, want, results[i])
		}

		// The singleflight run populated the cache
		cacheSubs, err := subsCache.GetFollows(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, want, cacheSubs)
	})

	t.Run("no followers in cache & repo, returns empty", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)
		redisClient := migrations.SetupRedis(ctx, t)

		userRepo := NewUserRepository(database.New(pool))
		subsCache := cache.NewSubsCacher(redisClient)
		wrapper := NewUserCacheWrapper(userRepo, subsCache)

		// Setup a user with no followers
		author := &database.User{
			ID:           uuid.New(),
			Login:        "getsubs_author4",
			Email:        "getsubs_author4@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))

		subs, err := wrapper.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Empty(t, subs)

		// An empty followers list cannot be represented as a redis set,
		// so the cache stays empty and the next read misses again
		_, err = subsCache.GetFollows(ctx, author.ID)
		assert.ErrorIs(t, err, errs.ErrNotFound)
	})
}
