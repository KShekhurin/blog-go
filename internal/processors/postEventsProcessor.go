package processors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

var ErrNothingToFetch = fmt.Errorf("nothing to fetch")

type PostEventsProcessor interface {
	Start()
	Close(ctx context.Context)
}

type postEventsProcessor struct {
	q          *database.Queries
	feedCacher cache.FeedCacher
	userRepo   repositories.UserRepository

	backoff Backoff
	limit   int32

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
		backoff:             NewExponentialBackOff(2, time.Second, 30*time.Second),
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
			return fmt.Errorf("unknown OutboxMessageType %s", event.MessageType)
		}
	}
	return nil
}

func (p *postEventsProcessor) fetchAndProcess(ctx context.Context) error {
	events, err := p.q.GetOutboxEvents(ctx, p.limit)
	if err != nil {
		return fmt.Errorf("fetch from the outbox failed: %w", err)
	}

	if len(events) == 0 {
		return ErrNothingToFetch
	}

	err = p.processEvents(ctx, events)
	if err != nil {
		return fmt.Errorf("event processing failed: %w", err)
	}

	err = p.q.SetEventsAsProcessed(ctx, fromEventsToIds(events))
	if err != nil {
		return fmt.Errorf("set events as processed failed: %w", err)
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
	const op = "PostEventsProcessor.ProcessLoop"
	ctx := context.Background()

	defer func() { close(p.processFinishedChan) }()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timer.Reset(p.backoff.NextDelay())

		select {
		case <-p.doneChan:
			return
		case <-timer.C:
			err := p.fetchAndProcess(ctx)
			if err != nil {
				if !errors.Is(err, ErrNothingToFetch) {
					slog.Error("fetch & process cycle failed", slog.String("op", op), slog.Any("err", err))
				}
				p.backoff.AccumulateAdverse()
			} else {
				p.backoff.ClearAdverse()
			}
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
