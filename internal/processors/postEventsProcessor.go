package processors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

type PostEventsProcessor interface {
	Start()
	Close(ctx context.Context)
}

type postEventsProcessor struct {
	feedCacher cache.FeedCacher
	userRepo   repositories.UserRepository

	limit int32

	q                   *database.Queries
	processFinishedChan chan struct{}
	doneChan            chan struct{}
}

func NewPostEventsProcessor(q *database.Queries, feedCacher cache.FeedCacher, userRepo repositories.UserRepository, limit int32) PostEventsProcessor {
	return &postEventsProcessor{
		q:                   q,
		doneChan:            make(chan struct{}, 1),
		processFinishedChan: make(chan struct{}, 1),
		feedCacher:          feedCacher,
		userRepo:            userRepo,
		limit:               limit,
	}
}

func (p *postEventsProcessor) removeFromFeed(ctx context.Context, post *webModels.Post) error {
	subs, err := p.userRepo.GetSubs(ctx, post.AuthorId)

	if err != nil {
		return err
	}

	err = p.feedCacher.RemoveFromFeeds(ctx, post.Id, post.CreatedAt, subs)
	if err != nil {
		return err
	}

	return nil
}

func (p *postEventsProcessor) pushToFeed(ctx context.Context, post *webModels.Post) error {
	subs, err := p.userRepo.GetSubs(ctx, post.AuthorId)

	if err != nil {
		return err
	}

	err = p.feedCacher.PushToFeeds(ctx, post.Id, post.CreatedAt, subs)
	if err != nil {
		return err
	}

	return nil
}

func (p *postEventsProcessor) processEvents(ctx context.Context, events []database.Outbox) error {
	for _, event := range events {
		switch event.MessageType {
		case database.OutboxMessageTypePostadded:
			err := p.pushToFeed(ctx, event.Payload)
			if err != nil {
				return err
			}
		case database.OutboxMessageTypePostdeleted:
			err := p.removeFromFeed(ctx, event.Payload)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown OutboxMessageType %d", event.MessageType)
		}
	}
	return nil
}

func fromEventsToIds(events []database.Outbox) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(events))

	for _, event := range events {
		ids = append(ids, event.ID)
	}

	return ids
}

func (p *postEventsProcessor) loop() {
	const op = "PostEventsProcessor.FetchEvents"
	ctx := context.Background()

	defer func() { close(p.processFinishedChan) }()

	backoff := NewExponentialBackOff(2, time.Second, 30*time.Second)
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timer.Reset(backoff.NextDelay())

		select {
		case <-p.doneChan:
			return
		case <-timer.C:
			events, err := p.q.GetOutboxEvents(ctx, p.limit)
			if err != nil {
				slog.Error("failed to fetch events from the outbox",
					slog.String("op", op),
					slog.Any("err", err))
				backoff.AccumulateAdverse()
				continue
			}

			if len(events) == 0 {
				backoff.AccumulateAdverse()
				continue
			}

			err = p.processEvents(ctx, events)
			if err != nil {
				slog.Error("failed to process events",
					slog.String("op", op),
					slog.Any("err", err))
				backoff.AccumulateAdverse()
				continue
			}

			err = p.q.SetEventsAsProcessed(
				ctx,
				fromEventsToIds(events))
			if err != nil {
				slog.Error("failed to set events as processed",
					slog.String("op", op),
					slog.Any("err", err))
				backoff.AccumulateAdverse()
				continue
			}

			backoff.ClearAdverse()
		}
	}
}

func (p *postEventsProcessor) Start() {
	go p.loop()
}

func (p *postEventsProcessor) Close(ctx context.Context) {
	close(p.doneChan)

	select {
	case <-p.processFinishedChan:
	case <-ctx.Done():
	}
}
