package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type FeedCacher interface {
	PushToFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error
	RemoveFromFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error
	GetFeedPostsIds(ctx context.Context, userId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]uuid.UUID, error)
}

type feedCacher struct {
	cache *redis.Client
}

func NewFeedCacher(cache *redis.Client) FeedCacher {
	return &feedCacher{
		cache: cache,
	}
}

func feedMember(createdAt time.Time, postId uuid.UUID) string {
	return fmt.Sprintf("%013d:%s", createdAt.UnixMilli(), postId.String())
}

func (c *feedCacher) PushToFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error {
	pipeline := c.cache.Pipeline()

	for _, subId := range subsIds {
		pipeline.ZAdd(ctx,
			fmt.Sprintf("user:%s:feed", subId),
			redis.Z{
				Score:  0, // identical for all members: ordering is purely lexicographic
				Member: feedMember(createdAt, postId),
			})
	}

	_, err := pipeline.Exec(ctx)

	return err
}

func (c *feedCacher) RemoveFromFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error {
	pipeline := c.cache.Pipeline()

	for _, subId := range subsIds {
		pipeline.ZRem(ctx,
			fmt.Sprintf("user:%s:feed", subId),
			feedMember(createdAt, postId),
		)
	}

	_, err := pipeline.Exec(ctx)

	return err
}

func (c *feedCacher) GetFeedPostsIds(ctx context.Context, userId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]uuid.UUID, error) {
	feedKey := fmt.Sprintf("user:%s:feed", userId)

	args := redis.ZRangeArgs{
		Key:   feedKey,
		Start: "+",
		Stop:  "-",
		Rev:   true,
		ByLex: true,
		Count: int64(limit),
	}
	if cursor != nil {
		// exclusive bound: everything strictly below the cursor in (created_at, uuid) order
		args.Start = "(" + feedMember(cursor.LastTime, cursor.LastId)
	}

	members, err := c.cache.ZRangeArgs(ctx, args).Result()
	if err != nil {
		return nil, err
	}

	postIds := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		_, idStr, _ := strings.Cut(m, ":")
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		postIds = append(postIds, id)
	}

	return postIds, nil
}
