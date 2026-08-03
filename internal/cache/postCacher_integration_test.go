//go:build integration

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setupPostCacherTest(t *testing.T) (PostCacher, *redis.Client, context.Context) {
	t.Helper()

	ctx := context.Background()
	client := migrations.SetupRedis(ctx, t)

	cacher := NewPostCacher(client, &CacherParams{TTL: time.Hour})

	return cacher, client, ctx
}

// postKey returns the redis key of the post.
func postKey(postId uuid.UUID) string {
	return fmt.Sprintf("post:%s", postId)
}

// newTestPost creates a non-deleted post with random ids.
func newTestPost() webModels.Post {
	return webModels.Post{
		Id:        uuid.New(),
		AuthorId:  uuid.New(),
		Content:   "test content",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
}

// putPostRaw stores the post in redis directly, bypassing the cacher.
func putPostRaw(t *testing.T, ctx context.Context, client *redis.Client, post webModels.Post) {
	t.Helper()

	err := client.JSONSet(ctx, postKey(post.Id), "$", post).Err()
	require.NoError(t, err)
}

// getPostRaw reads the post from redis directly, bypassing the cacher.
// Returns false when the key does not exist.
func getPostRaw(t *testing.T, ctx context.Context, client *redis.Client, postId uuid.UUID) (webModels.Post, bool) {
	t.Helper()

	res, err := client.JSONGet(ctx, postKey(postId), "$").Result()
	if err == redis.Nil {
		return webModels.Post{}, false
	}
	require.NoError(t, err)

	fmt.Println(res)

	var wrapped []webModels.Post
	require.NoError(t, json.Unmarshal([]byte(res), &wrapped))
	require.NotEmpty(t, wrapped)

	return wrapped[0], true
}

func TestRemovePostById(t *testing.T) {
	t.Run("post removed: deleted_at is set", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		post := newTestPost()
		putPostRaw(t, ctx, client, post)

		removeAt := time.Now().UTC().Truncate(time.Millisecond)
		require.NoError(t, cacher.RemovePostById(ctx, post.Id, removeAt))

		got, found := getPostRaw(t, ctx, client, post.Id)
		require.True(t, found)
		require.NotNil(t, got.DeletedAt)
		require.True(t, removeAt.Equal(*got.DeletedAt),
			"expected deleted_at %v, got %v", removeAt, *got.DeletedAt)
	})

	t.Run("post does not exist: not found error", func(t *testing.T) {
		t.Parallel()
		cacher, _, ctx := setupPostCacherTest(t)

		err := cacher.RemovePostById(ctx, uuid.New(), time.Now())
		require.ErrorIs(t, err, errs.ErrNotFound)
	})

	t.Run("post is already deleted: throws error", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		post := newTestPost()
		deletedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
		post.DeletedAt = &deletedAt
		putPostRaw(t, ctx, client, post)

		err := cacher.RemovePostById(ctx, post.Id, time.Now())
		require.ErrorIs(t, err, errs.ErrAlreadyDeleted)
	})
}

func TestAddPost(t *testing.T) {
	t.Run("post added", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		post := newTestPost()
		require.NoError(t, cacher.AddPost(ctx, &post))

		got, found := getPostRaw(t, ctx, client, post.Id)
		require.True(t, found)
		require.Equal(t, post.Id, got.Id)
		require.Equal(t, post.AuthorId, got.AuthorId)
		require.Equal(t, post.Content, got.Content)

		ttl, err := client.TTL(ctx, postKey(post.Id)).Result()
		require.NoError(t, err)
		require.Greater(t, ttl, time.Duration(0))
	})

	t.Run("post existed: overwritten", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		post := newTestPost()
		putPostRaw(t, ctx, client, post)

		post.Content = "overwritten content"
		require.NoError(t, cacher.AddPost(ctx, &post))

		got, found := getPostRaw(t, ctx, client, post.Id)
		require.True(t, found)
		require.Equal(t, "overwritten content", got.Content)
	})
}

func TestGetPostsWithIds(t *testing.T) {
	t.Run("all posts fetched", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		posts := []webModels.Post{newTestPost(), newTestPost(), newTestPost()}
		for _, p := range posts {
			putPostRaw(t, ctx, client, p)
		}

		ids := []uuid.UUID{posts[0].Id, posts[1].Id, posts[2].Id}
		fetched, missed, err := cacher.GetPostsWithIds(ctx, ids)
		require.NoError(t, err)
		require.Empty(t, missed)
		require.Len(t, fetched, 3)

		fetchedIds := make([]uuid.UUID, 0, len(fetched))
		for _, p := range fetched {
			fetchedIds = append(fetchedIds, p.Id)
		}
		require.ElementsMatch(t, ids, fetchedIds)
	})

	t.Run("some posts are missing: returned proper missing ids", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		post := newTestPost()
		putPostRaw(t, ctx, client, post)

		missingId := uuid.New()
		fetched, missed, err := cacher.GetPostsWithIds(ctx, []uuid.UUID{post.Id, missingId})
		require.NoError(t, err)

		require.Len(t, fetched, 1)
		require.Equal(t, post.Id, fetched[0].Id)
		require.Equal(t, []uuid.UUID{missingId}, missed)
	})

	t.Run("deleted posts are excluded both from found posts and from missing ones", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		alivePost := newTestPost()
		putPostRaw(t, ctx, client, alivePost)

		deletedPost := newTestPost()
		deletedAt := time.Now().UTC().Truncate(time.Millisecond)
		deletedPost.DeletedAt = &deletedAt
		putPostRaw(t, ctx, client, deletedPost)

		fetched, missed, err := cacher.GetPostsWithIds(ctx, []uuid.UUID{alivePost.Id, deletedPost.Id})
		require.NoError(t, err)

		require.Len(t, fetched, 1)
		require.Equal(t, alivePost.Id, fetched[0].Id)
		require.Empty(t, missed)
	})

	t.Run("no posts were found", func(t *testing.T) {
		t.Parallel()
		cacher, _, ctx := setupPostCacherTest(t)

		ids := []uuid.UUID{uuid.New(), uuid.New()}
		fetched, missed, err := cacher.GetPostsWithIds(ctx, ids)
		require.NoError(t, err)

		require.Empty(t, fetched)
		require.ElementsMatch(t, ids, missed)
	})
}

func TestAddPosts(t *testing.T) {
	t.Run("all posts are added", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		posts := []webModels.Post{newTestPost(), newTestPost(), newTestPost()}
		require.NoError(t, cacher.AddPosts(ctx, posts))

		for _, expected := range posts {
			got, found := getPostRaw(t, ctx, client, expected.Id)
			require.True(t, found)
			require.Equal(t, expected.Id, got.Id)
			require.Equal(t, expected.Content, got.Content)

			ttl, err := client.TTL(ctx, postKey(expected.Id)).Result()
			require.NoError(t, err)
			require.Greater(t, ttl, time.Duration(0))
		}
	})

	t.Run("no posts provided: no error", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupPostCacherTest(t)

		require.NoError(t, cacher.AddPosts(ctx, nil))

		keys, err := client.Keys(ctx, "post:*").Result()
		require.NoError(t, err)
		require.Empty(t, keys)
	})
}
