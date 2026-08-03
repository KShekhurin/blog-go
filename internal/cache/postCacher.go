package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type CacherParams struct {
	TTL time.Duration
}

type PostCacher interface {
	AddPost(ctx context.Context, post *webModels.Post) error
	AddPosts(ctx context.Context, posts []webModels.Post) error
	RemovePostById(ctx context.Context, postId uuid.UUID, removeAt time.Time) error
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

func (c *postCacher) RemovePostById(ctx context.Context, postId uuid.UUID, removeAt time.Time) error {
	//NOTE: It is a data race but its not critical in terms or caching

	key := fmt.Sprintf("post:%v", postId)
	exists, err := c.cache.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("post does not exist: %w",
			&errs.NotFoundError{
				ID:       key,
				Resource: "RemovePostById",
			})
	}

	result, err := c.cache.JSONGet(ctx, key, "$.deleted_at").Result()
	if err != nil {
		return err
	}

	if result != "[]" { //TODO: this is awful but JSONType is even worse
		return fmt.Errorf("could not delete post: %w", errs.ErrAlreadyDeleted)
	}

	err = c.cache.JSONSet(ctx, key, "$.deleted_at", removeAt).Err()

	return err
}

func (c *postCacher) AddPost(ctx context.Context, post *webModels.Post) error {
	key := fmt.Sprintf("post:%s", post.Id)

	pipe := c.cache.Pipeline()
	pipe.JSONSet(ctx, key, "$", *post)
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

	results, err := c.cache.JSONMGet(ctx, "$", keys...).Result()
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

		var wrappedPosts []webModels.Post
		if err := json.Unmarshal([]byte(jsonStr), &wrappedPosts); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal post %s: %w", ids[i], err)
		}

		if len(wrappedPosts) == 0 {
			missedPostsIds = append(missedPostsIds, ids[i])
			continue
		}

		if wrappedPosts[0].DeletedAt == nil { //Exclude deleted posts from user access
			posts = append(posts, wrappedPosts[0])
		}
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
