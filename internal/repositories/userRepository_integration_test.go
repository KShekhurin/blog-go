//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddUser(t *testing.T) {
	t.Run("user added", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)

		user := &database.User{
			ID:           uuid.New(),
			Login:        "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
		}

		err := userRepo.AddUser(ctx, user)
		require.NoError(t, err)

		// Verify user was actually persisted
		found, err := userRepo.FindUserById(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Login, found.Login)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.PasswordHash, found.PasswordHash)
	})

	t.Run("user already exists", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		q := database.New(pool)
		userRepo := NewUserRepository(q)

		user := &database.User{
			ID:           uuid.New(),
			Login:        "duplicate",
			Email:        "duplicate@example.com",
			PasswordHash: "hashed_password",
		}

		// First insert should succeed
		err := userRepo.AddUser(ctx, user)
		require.NoError(t, err)

		// Second insert with same credentials should fail
		err = userRepo.AddUser(ctx, user)
		assert.ErrorIs(t, err, ErrorUniqueViolation)
	})
}

func TestFindUserById(t *testing.T) {
	t.Run("user found", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		user := &database.User{
			ID:           uuid.New(),
			Login:        "byid_user",
			Email:        "byid@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, user))

		found, err := userRepo.FindUserById(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Login, found.Login)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.PasswordHash, found.PasswordHash)
	})

	t.Run("user not found, returns error", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		found, err := userRepo.FindUserById(ctx, uuid.New())
		assert.Nil(t, found)
		assert.ErrorIs(t, err, ErrorDoesNotExist)
	})
}

func TestFindUserByLogin(t *testing.T) {
	t.Run("user found", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		user := &database.User{
			ID:           uuid.New(),
			Login:        "bylogin_user",
			Email:        "bylogin@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, user))

		found, err := userRepo.FindUserByLogin(ctx, user.Login)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Login, found.Login)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.PasswordHash, found.PasswordHash)
	})

	t.Run("user not found, returns error", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		found, err := userRepo.FindUserByLogin(ctx, "missing_login")
		assert.Nil(t, found)
		assert.ErrorIs(t, err, ErrorDoesNotExist)
	})
}

func TestFindUserByLoginOrEmail(t *testing.T) {
	t.Run("found by login", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		user := &database.User{
			ID:           uuid.New(),
			Login:        "bylogin_combo",
			Email:        "combo_login@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, user))

		found, err := userRepo.FindUserByLoginOrEmail(ctx, user.Login, "other@example.com")
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Login, found.Login)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("found by email", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		user := &database.User{
			ID:           uuid.New(),
			Login:        "byemail_combo",
			Email:        "combo_email@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, user))

		found, err := userRepo.FindUserByLoginOrEmail(ctx, "missing_login", user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Login, found.Login)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("not found, error", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		found, err := userRepo.FindUserByLoginOrEmail(ctx, "missing_login", "missing@example.com")
		assert.Nil(t, found)
		assert.ErrorIs(t, err, ErrorDoesNotExist)
	})
}

func TestGetSubs(t *testing.T) {
	t.Run("returns non empty subs", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "subs_author",
			Email:        "subs_author@example.com",
			PasswordHash: "hashed_password",
		}
		sub1 := &database.User{
			ID:           uuid.New(),
			Login:        "subs_sub1",
			Email:        "subs_sub1@example.com",
			PasswordHash: "hashed_password",
		}
		sub2 := &database.User{
			ID:           uuid.New(),
			Login:        "subs_sub2",
			Email:        "subs_sub2@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub1))
		require.NoError(t, userRepo.AddUser(ctx, sub2))

		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub1.ID, author.ID))
		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub2.ID, author.ID))

		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{sub1.ID, sub2.ID}, subs)
	})

	t.Run("returns empty subs", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "subs_lonely",
			Email:        "subs_lonely@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))

		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestSubscribeUserTo(t *testing.T) {
	t.Run("successfully subscribed", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "sub_author",
			Email:        "sub_author@example.com",
			PasswordHash: "hashed_password",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "sub_sub",
			Email:        "sub_sub@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		err := userRepo.SubscribeUserTo(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify subscription was actually persisted
		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Contains(t, subs, sub.ID)
	})

	t.Run("already subscribed, returns error", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "dup_author",
			Email:        "dup_author@example.com",
			PasswordHash: "hashed_password",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "dup_sub",
			Email:        "dup_sub@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub.ID, author.ID))

		err := userRepo.SubscribeUserTo(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, ErrorUniqueViolation)
	})
}

func TestUnsubscribeUserFrom(t *testing.T) {
	t.Run("successfully unsubscribed", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "unsub_author",
			Email:        "unsub_author@example.com",
			PasswordHash: "hashed_password",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "unsub_sub",
			Email:        "unsub_sub@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		require.NoError(t, userRepo.SubscribeUserTo(ctx, sub.ID, author.ID))

		err := userRepo.UnsubscribeUserFrom(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify subscription was actually removed
		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.NotContains(t, subs, sub.ID)
	})

	t.Run("is not subscribed, throws error", func(t *testing.T) {
		ctx := context.Background()
		pool := migrations.SetupPostgres(ctx, t)

		userRepo := NewUserRepository(database.New(pool))

		author := &database.User{
			ID:           uuid.New(),
			Login:        "nosub_author",
			Email:        "nosub_author@example.com",
			PasswordHash: "hashed_password",
		}
		sub := &database.User{
			ID:           uuid.New(),
			Login:        "nosub_sub",
			Email:        "nosub_sub@example.com",
			PasswordHash: "hashed_password",
		}
		require.NoError(t, userRepo.AddUser(ctx, author))
		require.NoError(t, userRepo.AddUser(ctx, sub))

		err := userRepo.UnsubscribeUserFrom(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, ErrorDoesNotExist)
	})
}
