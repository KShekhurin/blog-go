package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/redis/go-redis/v9"
)

type CacherParams struct {
	TTL time.Duration
}

type PostCacher interface {
	AddPost(ctx context.Context, post *webModels.Post) error
	AddPosts(ctx context.Context, posts []webModels.Post) error
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
	pipe.JSONSet(ctx, key, "$", post)
	pipe.Expire(ctx, key, c.params.TTL)

	_, err := pipe.Exec(ctx)

	return err
}

//func (c *postCacher) GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, int, error) {
//
//}

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
