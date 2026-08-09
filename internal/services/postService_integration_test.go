//go:build integration

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

// setupPostService builds the service on top of the real repository
// wrapped with the redis-backed cache wrapper.
func setupPostService(t *testing.T) (PostService, repositories.PostRepository, *redis.Client, *pgxpool.Pool, context.Context) {
	t.Helper()

	ctx := context.Background()
	pool := migrations.SetupPostgres(ctx, t)
	redisClient := migrations.SetupRedis(ctx, t)

	postRepo := repositories.NewPostRepository(pool)
	postCache := cache.NewPostCacher(redisClient, &cache.CacherParams{TTL: time.Hour})
	wrappedRepo := repositories.NewPostCacheWrapper(postRepo, postCache)

	return NewPostService(wrappedRepo), postRepo, redisClient, pool, ctx
}

// mustCreatePostServiceAuthor inserts a user so posts satisfy the author_id foreign key.
func mustCreatePostServiceAuthor(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	userRepo := repositories.NewUserRepository(database.New(pool))
	require.NoError(t, userRepo.AddUser(ctx, &database.User{
		ID:           id,
		Login:        fmt.Sprintf("%s", id)[:20],
		PasswordHash: "password",
	}))

	return id
}

func postServiceCacheKey(postId uuid.UUID) string {
	return fmt.Sprintf("post:%s", postId)
}

// getCachedServicePost reads the post from redis directly, bypassing the wrapper.
// Returns false when the key does not exist.
func getCachedServicePost(t *testing.T, ctx context.Context, client *redis.Client, postId uuid.UUID) (webModels.Post, bool) {
	t.Helper()

	res, err := client.JSONGet(ctx, postServiceCacheKey(postId), "$").Result()
	if err == redis.Nil {
		return webModels.Post{}, false
	}
	require.NoError(t, err)

	var wrapped []webModels.Post
	require.NoError(t, json.Unmarshal([]byte(res), &wrapped))
	require.NotEmpty(t, wrapped)

	return wrapped[0], true
}

// mustCreateServicePost adds a post through the service (fills both db and cache).
func mustCreateServicePost(t *testing.T, ctx context.Context, service PostService, authorId uuid.UUID, content string) *webModels.Post {
	t.Helper()

	post, err := service.AddPost(ctx, webModels.CreatePostRequest{Content: content}, authorId)
	require.NoError(t, err)
	require.NotNil(t, post)

	return post
}

// --- RemovePostById ---

func TestPostService_RemovePostById(t *testing.T) {
	t.Run("post found and user authorized: deleted_at updated in db and cache", func(t *testing.T) {
		t.Parallel()
		service, postRepo, redisClient, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		post := mustCreateServicePost(t, ctx, service, authorId, "post to delete")

		removed, err := service.RemovePostById(ctx, post.Id, authorId)
		require.NoError(t, err)
		require.NotNil(t, removed)
		require.Equal(t, post.Id, removed.Id)
		require.NotNil(t, removed.DeletedAt)

		// deleted_at is persisted in postgres
		stored, err := postRepo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.NotNil(t, stored.DeletedAt)
		require.WithinDuration(t, *removed.DeletedAt, *stored.DeletedAt, time.Millisecond)

		// deleted_at is updated in the cache as well
		cached, found := getCachedServicePost(t, ctx, redisClient, post.Id)
		require.True(t, found)
		require.NotNil(t, cached.DeletedAt)
		require.WithinDuration(t, *removed.DeletedAt, *cached.DeletedAt, time.Millisecond)
	})

	t.Run("post not found: returns ErrorPostDoesNotExist", func(t *testing.T) {
		t.Parallel()
		service, _, _, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		removed, err := service.RemovePostById(ctx, uuid.New(), authorId)
		require.Nil(t, removed)
		require.ErrorIs(t, err, ErrorPostDoesNotExist)
	})

	t.Run("user is not the author: returns ErrorUnauthorized and post stays untouched", func(t *testing.T) {
		t.Parallel()
		service, postRepo, redisClient, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		post := mustCreateServicePost(t, ctx, service, authorId, "not yours")

		removed, err := service.RemovePostById(ctx, post.Id, uuid.New())
		require.Nil(t, removed)
		require.ErrorIs(t, err, ErrorUnauthorized)

		// post is not marked as deleted in postgres
		stored, err := postRepo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Nil(t, stored.DeletedAt)

		// ... and in the cache
		cached, found := getCachedServicePost(t, ctx, redisClient, post.Id)
		require.True(t, found)
		require.Nil(t, cached.DeletedAt)
	})
}

// --- AddPost ---

func TestPostService_AddPost(t *testing.T) {
	t.Run("post is created successfully and stored in db and cache", func(t *testing.T) {
		t.Parallel()
		service, postRepo, redisClient, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		request := webModels.CreatePostRequest{
			Content: "hello world",
			Attached: []webModels.CreateAttachedMedia{
				{
					Type:         "image",
					MimeType:     "image/png",
					Url:          "https://example.com/media/" + uuid.NewString() + ".png",
					DisplayOrder: 0,
				},
			},
		}

		post, err := service.AddPost(ctx, request, authorId)
		require.NoError(t, err)
		require.NotNil(t, post)
		require.NotEqual(t, uuid.Nil, post.Id)
		require.Equal(t, authorId, post.AuthorId)
		require.Equal(t, request.Content, post.Content)
		require.False(t, post.CreatedAt.IsZero())
		require.Nil(t, post.DeletedAt)
		require.Len(t, post.Attached, 1)
		require.Equal(t, post.Id, post.Attached[0].PostId)

		// persisted in postgres
		stored, err := postRepo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Equal(t, post.Id, stored.Id)
		require.Equal(t, request.Content, stored.Content)
		require.Len(t, stored.Attached, 1)

		// and warmed in the cache
		cached, found := getCachedServicePost(t, ctx, redisClient, post.Id)
		require.True(t, found)
		require.Equal(t, post.Id, cached.Id)
		require.Equal(t, request.Content, cached.Content)
	})
}

// --- GetPostById ---

func TestPostService_GetPostById(t *testing.T) {
	t.Run("post found and not deleted: returns it", func(t *testing.T) {
		t.Parallel()
		service, _, _, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		post := mustCreateServicePost(t, ctx, service, authorId, "fetch me")

		got, err := service.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, post.Id, got.Id)
		require.Equal(t, authorId, got.AuthorId)
		require.Equal(t, "fetch me", got.Content)
		require.Nil(t, got.DeletedAt)
	})

	t.Run("post not found: returns ErrorPostDoesNotExist", func(t *testing.T) {
		t.Parallel()
		service, _, _, _, ctx := setupPostService(t)

		got, err := service.GetPostById(ctx, uuid.New())
		require.Nil(t, got)
		require.ErrorIs(t, err, ErrorPostDoesNotExist)
	})

	t.Run("post was deleted: returns ErrorPostWasDeleted", func(t *testing.T) {
		t.Parallel()
		service, _, _, pool, ctx := setupPostService(t)
		authorId := mustCreatePostServiceAuthor(t, ctx, pool)

		post := mustCreateServicePost(t, ctx, service, authorId, "soon to be deleted")

		_, err := service.RemovePostById(ctx, post.Id, authorId)
		require.NoError(t, err)

		got, err := service.GetPostById(ctx, post.Id)
		require.Nil(t, got)
		require.ErrorIs(t, err, ErrorPostWasDeleted)
	})
}
