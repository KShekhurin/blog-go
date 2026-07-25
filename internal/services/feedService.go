package services

import (
	"context"
	"errors"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/workpool"
)

var (
	ErrorFeedQueueIsFull = errors.New("feed queue is full")
)

type FeedService interface {
	PushToFeeds(post *webModels.Post) error
	Close()
}

type FeedParams struct {
	Timeout  time.Duration
	WpParams *workpool.Params
}

type feedService struct {
	feedCacher cache.FeedCacher
	userRepo   repositories.UserRepository
	wp         workpool.WorkPool[*webModels.Post]
	feedParams *FeedParams
}

func NewFeedService(feedCacher cache.FeedCacher, userRepo repositories.UserRepository, feedParams *FeedParams) FeedService {
	s := &feedService{
		feedCacher: feedCacher,
		userRepo:   userRepo,
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
