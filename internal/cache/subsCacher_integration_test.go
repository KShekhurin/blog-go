//go:build integration

package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setupSubsCacherTest(t *testing.T) (SubsCacher, *redis.Client, context.Context) {
	t.Helper()

	ctx := context.Background()
	client := migrations.SetupRedis(ctx, t)

	return NewSubsCacher(client), client, ctx
}

// followersKey returns the redis key of the user's followers set.
func followersKey(userId uuid.UUID) string {
	return fmt.Sprintf("user:%s:followers", userId)
}

// subscriptionsKey returns the redis key of the user's subscriptions set.
func subscriptionsKey(userId uuid.UUID) string {
	return fmt.Sprintf("user:%s:subscriptions", userId)
}

func TestSubscribeUserTo(t *testing.T) {
	t.Run("adds follower and subscription successfully", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		subId := uuid.New()
		authId := uuid.New()

		existingFollower := uuid.New()
		existingSubscription := uuid.New()
		require.NoError(t, client.SAdd(ctx, followersKey(authId), existingFollower.String()).Err())
		require.NoError(t, client.SAdd(ctx, subscriptionsKey(subId), existingSubscription.String()).Err())

		require.NoError(t, cacher.SubscribeUserTo(ctx, subId, authId))

		followers, err := client.SMembers(ctx, followersKey(authId)).Result()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{subId.String(), existingFollower.String()}, followers)

		subscriptions, err := client.SMembers(ctx, subscriptionsKey(subId)).Result()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{authId.String(), existingSubscription.String()}, subscriptions)
	})

	t.Run("keys did not exist: added them", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		subId := uuid.New()
		authId := uuid.New()

		require.NoError(t, cacher.SubscribeUserTo(ctx, subId, authId))

		followers, err := client.SMembers(ctx, followersKey(authId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{subId.String()}, followers)

		subscriptions, err := client.SMembers(ctx, subscriptionsKey(subId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{authId.String()}, subscriptions)
	})
}

func TestUnsubscribeUserFrom(t *testing.T) {
	t.Run("successfully unsubscribed", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		subId := uuid.New()
		authId := uuid.New()
		otherFollower := uuid.New()
		otherSubscription := uuid.New()

		require.NoError(t, client.SAdd(ctx, followersKey(authId), subId.String(), otherFollower.String()).Err())
		require.NoError(t, client.SAdd(ctx, subscriptionsKey(subId), authId.String(), otherSubscription.String()).Err())

		require.NoError(t, cacher.UnsubscribeUserFrom(ctx, subId, authId))

		followers, err := client.SMembers(ctx, followersKey(authId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{otherFollower.String()}, followers)

		subscriptions, err := client.SMembers(ctx, subscriptionsKey(subId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{otherSubscription.String()}, subscriptions)
	})

	t.Run("keys did not exist: no error", func(t *testing.T) {
		cacher, _, ctx := setupSubsCacherTest(t)

		require.NoError(t, cacher.UnsubscribeUserFrom(ctx, uuid.New(), uuid.New()))
	})

	t.Run("there was no subscription anyway: no error", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		subId := uuid.New()
		authId := uuid.New()
		otherFollower := uuid.New()
		otherSubscription := uuid.New()

		require.NoError(t, client.SAdd(ctx, followersKey(authId), otherFollower.String()).Err())
		require.NoError(t, client.SAdd(ctx, subscriptionsKey(subId), otherSubscription.String()).Err())

		require.NoError(t, cacher.UnsubscribeUserFrom(ctx, subId, authId))

		followers, err := client.SMembers(ctx, followersKey(authId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{otherFollower.String()}, followers)

		subscriptions, err := client.SMembers(ctx, subscriptionsKey(subId)).Result()
		require.NoError(t, err)
		require.Equal(t, []string{otherSubscription.String()}, subscriptions)
	})
}

func TestGetSubs(t *testing.T) {
	t.Run("got all subs", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		userId := uuid.New()
		subs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

		members := make([]interface{}, 0, len(subs))
		for _, subId := range subs {
			members = append(members, subId.String())
		}
		require.NoError(t, client.SAdd(ctx, followersKey(userId), members...).Err())

		got, err := cacher.GetSubs(ctx, userId)
		require.NoError(t, err)
		require.ElementsMatch(t, subs, got)
	})

	t.Run("key had subs but all were removed: throws ErrorDoesNotExist", func(t *testing.T) {
		cacher, client, ctx := setupSubsCacherTest(t)

		userId := uuid.New()
		subId := uuid.New()

		// Redis deletes a set key when its last member is removed,
		// so an emptied set is indistinguishable from a missing key.
		require.NoError(t, client.SAdd(ctx, followersKey(userId), subId.String()).Err())
		require.NoError(t, client.SRem(ctx, followersKey(userId), subId.String()).Err())

		_, err := cacher.GetSubs(ctx, userId)
		require.ErrorIs(t, err, ErrorDoesNotExist)
	})

	t.Run("key does not exist: throws ErrorDoesNotExist", func(t *testing.T) {
		cacher, _, ctx := setupSubsCacherTest(t)

		_, err := cacher.GetSubs(ctx, uuid.New())
		require.ErrorIs(t, err, ErrorDoesNotExist)
	})
}
