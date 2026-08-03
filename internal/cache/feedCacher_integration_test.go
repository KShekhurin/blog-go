//go:build integration

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setupFeedCacherTest(t *testing.T) (FeedCacher, *redis.Client, context.Context) {
	t.Helper()

	ctx := context.Background()
	client := migrations.SetupRedis(ctx, t)

	return NewFeedCacher(client), client, ctx
}

// truncateMillis truncates to milliseconds because the feed member format
// stores createdAt with millisecond precision.
func truncateMillis(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

// feedKey returns the redis key of the user's feed.
func feedKey(userId uuid.UUID) string {
	return fmt.Sprintf("user:%s:feed", userId)
}

// pushTestPost pushes a post into a single user's feed directly via redis,
// simulating an already-populated feed.
func pushTestPost(t *testing.T, ctx context.Context, client *redis.Client, userId uuid.UUID, postId uuid.UUID, createdAt time.Time) {
	t.Helper()

	err := client.ZAdd(ctx, feedKey(userId), redis.Z{
		Score:  0,
		Member: feedMember(createdAt, postId),
	}).Err()
	require.NoError(t, err)
}

// feedMembers returns raw members of the user's feed.
func feedMembers(t *testing.T, ctx context.Context, client *redis.Client, userId uuid.UUID) []string {
	t.Helper()

	members, err := client.ZRange(ctx, feedKey(userId), 0, -1).Result()
	require.NoError(t, err)
	return members
}

func TestPushToFeeds(t *testing.T) {
	t.Run("pushes to all listed feeds", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		postId := uuid.New()
		createdAt := truncateMillis(time.Now())
		subsIds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

		require.NoError(t, cacher.PushToFeeds(ctx, postId, createdAt, subsIds))

		expected := feedMember(createdAt, postId)
		for _, subId := range subsIds {
			members := feedMembers(t, ctx, client, subId)
			require.Equal(t, []string{expected}, members)
		}
	})

	t.Run("no sub ids provided: no error and nothing pushed", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		require.NoError(t, cacher.PushToFeeds(ctx, uuid.New(), truncateMillis(time.Now()), nil))

		keys, err := client.Keys(ctx, "user:*:feed").Result()
		require.NoError(t, err)
		require.Empty(t, keys)
	})
}

func TestRemoveFromFeeds(t *testing.T) {
	t.Run("removes from all listed feeds", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		postId := uuid.New()
		createdAt := truncateMillis(time.Now())
		subsIds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

		for _, subId := range subsIds {
			pushTestPost(t, ctx, client, subId, postId, createdAt)
		}

		require.NoError(t, cacher.RemoveFromFeeds(ctx, postId, createdAt, subsIds))

		for _, subId := range subsIds {
			require.Empty(t, feedMembers(t, ctx, client, subId))
		}
	})

	t.Run("no error when feed key does not contain the member", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		subId := uuid.New()
		// feed exists but holds a different post
		pushTestPost(t, ctx, client, subId, uuid.New(), truncateMillis(time.Now()))

		require.NoError(t, cacher.RemoveFromFeeds(ctx, uuid.New(), truncateMillis(time.Now()), []uuid.UUID{subId}))

		// the other member is untouched
		require.Len(t, feedMembers(t, ctx, client, subId), 1)
	})

	t.Run("no error when feed key does not exist at all", func(t *testing.T) {
		t.Parallel()
		cacher, _, ctx := setupFeedCacherTest(t)

		err := cacher.RemoveFromFeeds(ctx, uuid.New(), truncateMillis(time.Now()), []uuid.UUID{uuid.New()})
		require.NoError(t, err)
	})

	t.Run("no ids provided: no error", func(t *testing.T) {
		t.Parallel()
		cacher, _, ctx := setupFeedCacherTest(t)

		require.NoError(t, cacher.RemoveFromFeeds(ctx, uuid.New(), truncateMillis(time.Now()), nil))
	})
}

func TestGetFeedPostsIds(t *testing.T) {
	base := truncateMillis(time.Now())

	t.Run("no cursor: returns newest posts first up to limit", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		userId := uuid.New()
		postIds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
		for i, id := range postIds {
			pushTestPost(t, ctx, client, userId, id, base.Add(time.Duration(i)*time.Minute))
		}

		got, err := cacher.GetFeedPostsIds(ctx, userId, nil, 3)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{postIds[3], postIds[2], postIds[1]}, got)
	})

	t.Run("with cursor: returns posts strictly before the cursor position", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		userId := uuid.New()
		postIds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
		for i, id := range postIds {
			pushTestPost(t, ctx, client, userId, id, base.Add(time.Duration(i)*time.Minute))
		}

		cursor := &webModels.PostPaginationCursor{
			LastId:   postIds[2],
			LastTime: base.Add(2 * time.Minute),
		}

		got, err := cacher.GetFeedPostsIds(ctx, userId, cursor, 10)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{postIds[1], postIds[0]}, got)
	})

	t.Run("feed with such user id does not exist: returns empty result without error", func(t *testing.T) {
		t.Parallel()
		cacher, _, ctx := setupFeedCacherTest(t)

		got, err := cacher.GetFeedPostsIds(ctx, uuid.New(), nil, 10)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("cursor points to the oldest post: returns empty result", func(t *testing.T) {
		t.Parallel()
		cacher, client, ctx := setupFeedCacherTest(t)

		userId := uuid.New()
		postIds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
		for i, id := range postIds {
			pushTestPost(t, ctx, client, userId, id, base.Add(time.Duration(i)*time.Minute))
		}

		cursor := &webModels.PostPaginationCursor{
			LastId:   postIds[0],
			LastTime: base,
		}

		got, err := cacher.GetFeedPostsIds(ctx, userId, cursor, 10)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}
