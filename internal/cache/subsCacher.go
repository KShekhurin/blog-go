package cache

import (
	"context"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type SubsCacher interface {
	SubscribeUserTo(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error
	UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error
	FollowListExists(ctx context.Context, authId uuid.UUID) (bool, error)
	SubsListExists(ctx context.Context, subId uuid.UUID) (bool, error)
	SetFollows(ctx context.Context, authId uuid.UUID, subIds []uuid.UUID) error
	GetFollows(ctx context.Context, authId uuid.UUID) ([]uuid.UUID, error)
}

type subsCacher struct {
	cache *redis.Client
}

func NewSubsCacher(client *redis.Client) SubsCacher {
	return &subsCacher{
		cache: client,
	}
}

func (c *subsCacher) FollowListExists(ctx context.Context, authId uuid.UUID) (bool, error) {
	subsKey := fmt.Sprintf("user:%s:followers", authId)

	exists, err := c.cache.Exists(ctx, subsKey).Result()

	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

func (c *subsCacher) SubsListExists(ctx context.Context, subId uuid.UUID) (bool, error) {
	//TODO implement me
	panic("implement me")
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

func (c *subsCacher) GetFollows(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	subsKey := fmt.Sprintf("user:%s:followers", userId)

	exists, err := c.cache.Exists(ctx, subsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("could not check subs key existence: %w", err)
	}
	if exists == 0 {
		return []uuid.UUID{}, &errs.NotFoundError{
			ID:       subsKey,
			Resource: "GetFollows",
		}
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

func (c *subsCacher) SetFollows(ctx context.Context, authId uuid.UUID, subIds []uuid.UUID) error {
	if len(subIds) == 0 {
		return nil
	}

	followsKey := fmt.Sprintf("user:%s:followers", authId)

	subIdsStr := make([]interface{}, 0, len(subIds))
	for _, subId := range subIds {
		subIdsStr = append(subIdsStr, subId.String())
	}

	err := c.cache.SAdd(ctx, followsKey, subIdsStr...).Err()

	return err
}
