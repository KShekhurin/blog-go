package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrorDoesNotExist = errors.New("does not exist")
)

type SubsCacher interface {
	SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
	UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
	GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error)
}

type subsCacher struct {
	cache *redis.Client
}

func NewSubsCacher(client *redis.Client) SubsCacher {
	return &subsCacher{
		cache: client,
	}
}

func (c *subsCacher) SubscribeUserTo(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	subsKey := fmt.Sprintf("user:%s:followers", authId)
	subscriptionKey := fmt.Sprintf("user:%s:subscriptions", subId)

	pipeline := c.cache.TxPipeline()

	pipeline.SAdd(ctx, subsKey, subId.String())
	pipeline.SAdd(ctx, subscriptionKey, authId.String())

	_, err := pipeline.Exec(ctx)

	return err
}

func (c *subsCacher) UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	subsKey := fmt.Sprintf("user:%s:followers", authId)
	subscriptionKey := fmt.Sprintf("user:%s:subscriptions", subId)

	pipeline := c.cache.TxPipeline()

	pipeline.SRem(ctx, subsKey, subId.String())
	pipeline.SRem(ctx, subscriptionKey, authId.String())

	_, err := pipeline.Exec(ctx)

	return err
}

func (c *subsCacher) GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	subsKey := fmt.Sprintf("user:%s:followers", userId)

	exists, err := c.cache.Exists(ctx, subsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("could not check subs key existence: %w", err)
	}
	if exists == 0 {
		return []uuid.UUID{}, ErrorDoesNotExist
	}

	res, err := c.cache.SMembers(ctx, subsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("could not get subs from cache: %w", err)
	}

	subIds := make([]uuid.UUID, 0, len(res))
	for _, id := range res {
		subId, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("could not parse sub id: %w", err)
		}
		subIds = append(subIds, subId)
	}

	return subIds, nil
}
