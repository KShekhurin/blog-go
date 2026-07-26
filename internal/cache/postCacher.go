package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrorPostFetchFailed = errors.New("post fetch failed")
)

type CacherParams struct {
	TTL time.Duration
}

type PostCacher interface {
	AddPost(ctx context.Context, post *webModels.Post) error
	AddPosts(ctx context.Context, posts []webModels.Post) error
	GetPostsWithIds(ctx context.Context, ids []uuid.UUID) (fetchedPosts []webModels.Post, missedPostsIds []uuid.UUID, err error)
}

type postCacher struct {
	cache  *redis.Client
	params *CacherParams
}

func NewPostCacher(cache *redis.Client, params *CacherParams) PostCacher {
	return &postCacher{
		cache:  cache,
		params: params,
	}
}

func (c *postCacher) AddPost(ctx context.Context, post *webModels.Post) error {
	key := fmt.Sprintf("post:%s", post.Id)

	pipe := c.cache.Pipeline()
	pipe.JSONSet(ctx, key, ".", *post)
	pipe.Expire(ctx, key, c.params.TTL)

	_, err := pipe.Exec(ctx)

	return err
}

func (c *postCacher) GetPostsWithIds(ctx context.Context, ids []uuid.UUID) (fetchedPosts []webModels.Post, missedPostsIds []uuid.UUID, err error) {
	if len(ids) == 0 {
		return []webModels.Post{}, []uuid.UUID{}, nil
	}

	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, fmt.Sprintf("post:%s", id))
	}

	results, err := c.cache.JSONMGet(ctx, ".", keys...).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("post fetch failed: %w", err)
	}

	posts := make([]webModels.Post, 0, len(results))
	missedPostsIds = make([]uuid.UUID, 0)

	for i, result := range results {
		if result == nil {
			missedPostsIds = append(missedPostsIds, ids[i])
			continue
		}

		jsonStr, ok := result.(string)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected type for post %s: %T", ids[i], result)
		}

		var post webModels.Post
		if err := json.Unmarshal([]byte(jsonStr), &post); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal post %s: %w", ids[i], err)
		}

		posts = append(posts, post)
	}

	return posts, missedPostsIds, nil
}

func (c *postCacher) AddPosts(ctx context.Context, posts []webModels.Post) error {
	pipe := c.cache.Pipeline()

	for _, post := range posts {
		key := fmt.Sprintf("post:%s", post.Id)
		pipe.JSONSet(ctx, key, "$", post)
		pipe.Expire(ctx, key, c.params.TTL)
	}

	_, err := pipe.Exec(ctx)

	return err
}
