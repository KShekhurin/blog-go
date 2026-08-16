package services

import (
	"context"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

type FeedService interface {
	GetFromFeed(ctx context.Context, userId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error)
}

type feedService struct {
	feedCacher cache.FeedCacher
	userRepo   repositories.UserRepository
	postRepo   repositories.PostRepository
}

func NewFeedService(feedCacher cache.FeedCacher, userRepo repositories.UserRepository, postRepo repositories.PostRepository) FeedService {
	s := &feedService{
		feedCacher: feedCacher,
		userRepo:   userRepo,
		postRepo:   postRepo,
	}

	return s
}

func makeNewCursor(posts []webModels.Post) *webModels.PostPaginationCursor {
	if len(posts) == 0 {
		return nil
	}

	lastPost := posts[len(posts)-1]

	return &webModels.PostPaginationCursor{
		LastId:   lastPost.Id,
		LastTime: lastPost.CreatedAt,
	}
}

func (s *feedService) GetFromFeed(ctx context.Context, userId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error) {
	const op = "FeedService.GetFromFeed"

	ctx, span := tracer.Start(ctx, op)
	defer span.End()

	postIds, err := s.feedCacher.GetFeedPostsIds(ctx, userId, cursor, limit)

	if err != nil {
		return nil, nil, err
	}

	posts, err := s.postRepo.GetPostsWithIds(ctx, postIds)
	if err != nil {
		return nil, nil, err
	}

	return posts, makeNewCursor(posts), nil
}
