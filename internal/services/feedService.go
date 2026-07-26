package services

import (
	"context"
	"errors"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/workpool"
	"github.com/google/uuid"
)

var (
	ErrorFeedQueueIsFull = errors.New("feed queue is full")
)

type FeedService interface {
	PushToFeeds(post *webModels.Post) error
	GetFromFeed(ctx context.Context, userId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error)
	Close()
}

type FeedParams struct {
	Timeout  time.Duration
	WpParams *workpool.Params
}

type feedService struct {
	feedCacher cache.FeedCacher
	userRepo   repositories.UserRepository
	postRepo   repositories.PostRepository
	wp         workpool.WorkPool[*webModels.Post]
	feedParams *FeedParams
}

func NewFeedService(feedCacher cache.FeedCacher, userRepo repositories.UserRepository, postRepo repositories.PostRepository, feedParams *FeedParams) FeedService {
	s := &feedService{
		feedCacher: feedCacher,
		userRepo:   userRepo,
		postRepo:   postRepo,
		feedParams: feedParams,
	}

	wp := workpool.NewWorkPool(feedParams.WpParams, s.pushToFeedHandle)

	s.wp = wp

	return s
}

func (s *feedService) Close() {
	s.wp.Close()
}

func (s *feedService) pushToFeedHandle(post *webModels.Post) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		s.feedParams.Timeout)
	defer cancel()

	subs, err := s.userRepo.GetSubs(ctx, post.AuthorId)

	if err != nil {
		return
	}

	err = s.feedCacher.PushToFeeds(ctx, post.Id, post.CreatedAt, subs)
}

func (s *feedService) PushToFeeds(post *webModels.Post) error {
	if !s.wp.TryPush(post) {
		return ErrorFeedQueueIsFull
	}
	return nil
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
