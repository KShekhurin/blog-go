package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type FeedCacher interface {
	PushToFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error
}

type feedCacher struct {
	cache *redis.Client
}

func NewFeedCacher(cache *redis.Client) FeedCacher {
	return &feedCacher{
		cache: cache,
	}
}

func (c feedCacher) PushToFeeds(ctx context.Context, postId uuid.UUID, createdAt time.Time, subsIds []uuid.UUID) error {
	pipeline := c.cache.Pipeline()

	for _, subId := range subsIds {
		pipeline.ZAdd(ctx,
			fmt.Sprintf("user:%s:feed", subId),
			redis.Z{
				Score:  float64(createdAt.Unix()),
				Member: postId.String(),
			})
	}

	_, err := pipeline.Exec(ctx)

	return err
}
