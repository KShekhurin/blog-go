package services

import (
	"context"
	"errors"
	"log/slog"
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

type state int

const (
	pushToFeed = iota
	removeFromFeed
)

type postAction struct {
	action state
	post   *webModels.Post
}

type FeedService interface {
	PushToFeeds(post *webModels.Post)
	RemoveFromFeeds(post *webModels.Post)
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
	wp         workpool.WorkPool[postAction]
	feedParams *FeedParams
}

func NewFeedService(feedCacher cache.FeedCacher, userRepo repositories.UserRepository, postRepo repositories.PostRepository, feedParams *FeedParams) FeedService {
	s := &feedService{
		feedCacher: feedCacher,
		userRepo:   userRepo,
		postRepo:   postRepo,
		feedParams: feedParams,
	}

	wp := workpool.NewWorkPool(feedParams.WpParams, s.handlePost)

	s.wp = wp

	return s
}

func (s *feedService) Close() {
	s.wp.Close()
}

func (s *feedService) handlePost(postAct postAction) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		s.feedParams.Timeout)

	defer cancel()

	switch postAct.action {
	case pushToFeed:
		s.pushToFeedHandle(ctx, postAct.post)
	case removeFromFeed:
		s.removeFromFeed(ctx, postAct.post)
	}

}

func (s *feedService) removeFromFeed(ctx context.Context, post *webModels.Post) {
	subs, err := s.userRepo.GetSubs(ctx, post.AuthorId)

	if err != nil {
		return
	}

	err = s.feedCacher.RemoveFromFeeds(ctx, post.Id, post.CreatedAt, subs)
	if err != nil {
		return
	}
}

func (s *feedService) pushToFeedHandle(ctx context.Context, post *webModels.Post) {
	subs, err := s.userRepo.GetSubs(ctx, post.AuthorId)

	if err != nil {
		return
	}

	err = s.feedCacher.PushToFeeds(ctx, post.Id, post.CreatedAt, subs)
}

func (s *feedService) PushToFeeds(post *webModels.Post) {
	if !s.wp.TryPush(postAction{pushToFeed, post}) {
		slog.Error("failed to push post to feed", slog.Any("err", ErrorFeedQueueIsFull))
	}
}

func (s *feedService) RemoveFromFeeds(post *webModels.Post) {
	if !s.wp.TryPush(postAction{removeFromFeed, post}) {
		slog.Error("failed to push post to feed", slog.Any("err", ErrorFeedQueueIsFull))
	}
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
