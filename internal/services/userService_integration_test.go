//go:build integration

package services

import (
	"context"
	"testing"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testHashParams = &argon2id.Params{
	Memory:      64 * 1024,
	Iterations:  1,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func setupUserService(ctx context.Context, t testing.TB) (UserService, repositories.UserRepository) {
	t.Helper()

	pool := migrations.SetupPostgres(ctx, t)
	redisClient := migrations.SetupRedis(ctx, t)

	userRepo := repositories.NewUserRepository(database.New(pool))
	subsCacher := cache.NewSubsCacher(redisClient)
	cachedUserRepo := repositories.NewUserCacheWrapper(userRepo, subsCacher)

	service := NewUserService(cachedUserRepo, testHashParams)

	return service, cachedUserRepo
}

func addTestUser(ctx context.Context, t testing.TB, repo repositories.UserRepository, login, email string) *database.User {
	t.Helper()

	user := &database.User{
		ID:           uuid.New(),
		Login:        login,
		Email:        email,
		PasswordHash: "hashed_password",
	}
	require.NoError(t, repo.AddUser(ctx, user))

	return user
}

func TestCreateUser(t *testing.T) {
	t.Run("no user with such login/email successfully creates one", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, _ := setupUserService(ctx, t)

		registerInfo := &webModels.UserRegisterInfo{
			Login:    "new_user",
			Email:    "new_user@example.com",
			Password: "secure_password",
		}

		created, err := service.CreateUser(ctx, registerInfo)
		require.NoError(t, err)
		require.NotNil(t, created)

		assert.NotEqual(t, uuid.Nil, created.ID)
		assert.Equal(t, registerInfo.Login, created.Login)
		assert.Equal(t, registerInfo.Email, created.Email)
		assert.NotEmpty(t, created.PasswordHash)
		assert.NotEqual(t, registerInfo.Password, created.PasswordHash)

		// Verify user was actually persisted
		found, err := service.GetUserByLoginOrEmail(ctx, registerInfo.Login, registerInfo.Email)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Login, found.Login)
		assert.Equal(t, created.Email, found.Email)
	})

	t.Run("user with such login exists, throws ErrUserAlreadyExists", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		addTestUser(ctx, t, userRepo, "taken_login", "first@example.com")

		registerInfo := &webModels.UserRegisterInfo{
			Login:    "taken_login",
			Email:    "second@example.com",
			Password: "secure_password",
		}

		created, err := service.CreateUser(ctx, registerInfo)
		assert.Nil(t, created)
		assert.ErrorIs(t, err, ErrUserAlreadyExists)
	})

	t.Run("user with such email exists, throws ErrUserAlreadyExists", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		addTestUser(ctx, t, userRepo, "first_login", "taken@example.com")

		registerInfo := &webModels.UserRegisterInfo{
			Login:    "second_login",
			Email:    "taken@example.com",
			Password: "secure_password",
		}

		created, err := service.CreateUser(ctx, registerInfo)
		assert.Nil(t, created)
		assert.ErrorIs(t, err, ErrUserAlreadyExists)
	})
}

func TestSubscribeTo(t *testing.T) {
	t.Run("user successfully subscribes to other user", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		author := addTestUser(ctx, t, userRepo, "sub_author", "sub_author@example.com")
		sub := addTestUser(ctx, t, userRepo, "sub_subscriber", "sub_subscriber@example.com")

		err := service.SubscribeTo(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify subscription was actually persisted
		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.Contains(t, subs, sub.ID)
	})

	t.Run("user already subscribed, throws ErrAlreadySubscribed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		author := addTestUser(ctx, t, userRepo, "dup_author", "dup_author@example.com")
		sub := addTestUser(ctx, t, userRepo, "dup_subscriber", "dup_subscriber@example.com")

		require.NoError(t, service.SubscribeTo(ctx, sub.ID, author.ID))

		err := service.SubscribeTo(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, ErrAlreadySubscribed)
	})
}

func TestUnsubscribeFrom(t *testing.T) {
	t.Run("successfully unsubscribed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		author := addTestUser(ctx, t, userRepo, "unsub_author", "unsub_author@example.com")
		sub := addTestUser(ctx, t, userRepo, "unsub_subscriber", "unsub_subscriber@example.com")

		require.NoError(t, service.SubscribeTo(ctx, sub.ID, author.ID))

		err := service.UnsubscribeFrom(ctx, sub.ID, author.ID)
		require.NoError(t, err)

		// Verify subscription was actually removed
		subs, err := userRepo.GetSubs(ctx, author.ID)
		require.NoError(t, err)
		assert.NotContains(t, subs, sub.ID)
	})

	t.Run("no subscription in the first place, throws ErrNotSubscribed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		service, userRepo := setupUserService(ctx, t)

		author := addTestUser(ctx, t, userRepo, "nosub_author", "nosub_author@example.com")
		sub := addTestUser(ctx, t, userRepo, "nosub_subscriber", "nosub_subscriber@example.com")

		err := service.UnsubscribeFrom(ctx, sub.ID, author.ID)
		assert.ErrorIs(t, err, ErrNotSubscribed)
	})
}
