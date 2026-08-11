package repositories

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/errs"
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

func (w *postCacheWrapper) RemovePostById(ctx context.Context, postId uuid.UUID, removeAt time.Time) error {
	const op = "PostCacheWrapper.RemovePostById"

	ctx, span := tracer.Start(ctx, op)
	defer span.End()

	err := w.postRepo.RemovePostById(ctx, postId, removeAt)
	if err != nil {
		return fmt.Errorf("could not delete post: %w", err)
	}

	err = w.postCache.RemovePostById(ctx, postId, removeAt)
	if err != nil && !errors.Is(err, errs.ErrNotFound) {
		// If we cannot access the cache then something bad has happened
		// Probably failure or cache is overloaded
		// Yet is it not an error from user perspective as db was updated
		// We should drop the key as it is now not in sync with db,
		// Deleted posts should not be accessible till the end of TTL

		span.RecordError(err)
		slog.ErrorContext(ctx, "removal of post from cache failed",
			slog.String("op", op),
			slog.Any("err", err))
	}

	return nil
}

func (w *postCacheWrapper) AddPost(ctx context.Context, post *webModels.Post) error {
	const op = "PostCacheWrapper.AddPost"

	ctx, span := tracer.Start(ctx, op)
	defer span.End()

	err := w.postRepo.AddPost(ctx, post)

	if err != nil {
		return err
	}

	err = w.postCache.AddPost(ctx, post)
	if err != nil {
		// If we cannot access the cache then something bad has happened
		// Probably failure or cache is overloaded
		// Yet is it not an error from user perspective as db was updated

		span.RecordError(err)
		slog.ErrorContext(ctx, "addition of post to cache failed",
			slog.String("op", op),
			slog.Any("err", err))
	}

	return nil
}

func (w *postCacheWrapper) GetPostsWithIds(ctx context.Context, ids []uuid.UUID) ([]webModels.Post, error) {
	const op = "PostCacheWrapper.GetPostsWithIds"

	ctx, span := tracer.Start(ctx, op)
	defer span.End()

	cachedPosts, missedPostsIds, err := w.postCache.GetPostsWithIds(ctx, ids)

	if err != nil {
		// If we cannot access the cache or data cannot be unmarshaled then something bad has happened
		// Probably failure or cache is overloaded
		// Yet is it not an error from user perspective as we can still access database

		span.RecordError(err)
		slog.ErrorContext(ctx, "read of cached posts failed",
			slog.String("op", op),
			slog.Any("err", err))

		missedPostsIds = ids
	}

	if len(missedPostsIds) > 0 {
		missedPosts, err := w.postRepo.GetPostsWithIds(ctx, missedPostsIds)
		if err != nil {
			return nil, err
		}

		//We should propagate fetched posts to the cache
		if err := w.postCache.AddPosts(ctx, missedPosts); err != nil {
			span.RecordError(err)
			slog.ErrorContext(ctx, "addition of posts to cache failed",
				slog.String("op", op),
				slog.Any("err", err))
		}

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
