package repositories

import (
	"cmp"
	"context"
	"slices"

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

func (w *postCacheWrapper) GetPostsWithIds(ctx context.Context, ids []uuid.UUID) ([]webModels.Post, error) {
	cachedPosts, missedPostsIds, err := w.postCache.GetPostsWithIds(ctx, ids)

	if err != nil {
		return nil, err
	}

	if len(missedPostsIds) > 0 {
		missedPosts, err := w.postRepo.GetPostsWithIds(ctx, missedPostsIds)
		if err != nil {
			return nil, err
		}
		//TODO: This is bad
		cachedPosts = append(cachedPosts, missedPosts...)
		cachedPosts = slices.SortedFunc(slices.Values(cachedPosts), func(a, b webModels.Post) int {
			if a.CreatedAt.Unix() != b.CreatedAt.Unix() {
				return cmp.Compare(a.CreatedAt.Unix(), b.CreatedAt.Unix())
			}
			return cmp.Compare(a.Id.String(), b.Id.String())
		})
	}

	return cachedPosts, nil
}

func (w *postCacheWrapper) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	return w.postRepo.GetPostById(ctx, id)
}

func (w *postCacheWrapper) GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error) {
	return w.postRepo.GetPostsByAuthorId(ctx, authorId, cursor, limit)
}
