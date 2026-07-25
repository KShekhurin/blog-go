package repositories

import (
	"context"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

type postCacheWrapper struct {
	postCache cache.PostCacher
	postRepo  PostRepository
}

func NewPostCacheWrapper(postRepo PostRepository, postCache cache.PostCacher) PostRepository {
	return &postCacheWrapper{
		postRepo:  postRepo,
		postCache: postCache,
	}
}

func (w *postCacheWrapper) AddPost(ctx context.Context, post *webModels.Post) error {
	err := w.postRepo.AddPost(ctx, post)

	if err != nil {
		return err
	}

	err = w.postCache.AddPost(ctx, post)
	return err //TODO: cache failure != db failure
}

func (w *postCacheWrapper) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	return w.postRepo.GetPostById(ctx, id)
}

func (w *postCacheWrapper) GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error) {
	return w.postRepo.GetPostsByAuthorId(ctx, authorId, cursor, limit)
}
