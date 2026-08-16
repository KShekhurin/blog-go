//go:build integration

package processors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type processorTestFixture struct {
	ctx         context.Context
	pool        *pgxpool.Pool
	redisClient *redis.Client
	q           *database.Queries
	userRepo    repositories.UserRepository
	feedCacher  cache.FeedCacher
}

func setupProcessorTest(t *testing.T) *processorTestFixture {
	t.Helper()

	ctx := context.Background()
	pool := migrations.SetupPostgres(ctx, t)
	redisClient := migrations.SetupRedis(ctx, t)

	q := database.New(pool)

	return &processorTestFixture{
		ctx:         ctx,
		pool:        pool,
		redisClient: redisClient,
		q:           q,
		// plain userRepository, no cache wrapper: internal logic must not rely on cache
		userRepo:   repositories.NewUserRepository(q),
		feedCacher: cache.NewFeedCacher(redisClient),
	}
}

func (f *processorTestFixture) newProcessor(t *testing.T, limit int32) *postEventsProcessor {
	t.Helper()
	return NewPostEventsProcessor(f.q, f.feedCacher, f.userRepo, limit).(*postEventsProcessor)
}

func (f *processorTestFixture) addUser(t *testing.T, login string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	err := f.userRepo.AddUser(f.ctx, &database.User{
		ID:           id,
		Login:        login,
		Email:        login + "@example.com",
		PasswordHash: "hashed_password",
	})
	require.NoError(t, err)
	return id
}

func (f *processorTestFixture) enqueueEvent(t *testing.T, messageType database.OutboxMessageType, post *webModels.Post) {
	t.Helper()

	require.NoError(t, f.q.AddToOutbox(f.ctx, database.AddToOutboxParams{
		MessageType: messageType,
		Payload:     post,
	}))
}

// outboxCounts returns how many events are still unprocessed and how many are
// processed.
func (f *processorTestFixture) outboxCounts(t *testing.T) (unprocessed, processed int) {
	t.Helper()

	err := f.pool.QueryRow(f.ctx,
		`SELECT
			COUNT(*) FILTER (WHERE processed_at IS NULL),
			COUNT(*) FILTER (WHERE processed_at IS NOT NULL)
		FROM outbox`,
	).Scan(&unprocessed, &processed)
	require.NoError(t, err)
	return
}

func (f *processorTestFixture) feedMembers(t *testing.T, userId uuid.UUID) []string {
	t.Helper()

	members, err := f.redisClient.ZRange(f.ctx, feedKey(userId), 0, -1).Result()
	require.NoError(t, err)
	return members
}

func feedKey(userId uuid.UUID) string {
	return fmt.Sprintf("user:%s:feed", userId)
}

// feedMember mirrors the member format of cache.feedCacher; kept local so the
// test pins the contract between processor and cache.
func feedMember(post *webModels.Post) string {
	return fmt.Sprintf("%013d:%s", post.CreatedAt.UnixMilli(), post.Id.String())
}

// newTestPost truncates CreatedAt to milliseconds, matching the precision the
// feed member format stores.
func newTestPost(authorId uuid.UUID) *webModels.Post {
	return &webModels.Post{
		Id:        uuid.New(),
		AuthorId:  authorId,
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestFetchAndProcessNothingToFetch(t *testing.T) {
	t.Parallel()

	f := setupProcessorTest(t)
	p := f.newProcessor(t, 10)

	err := p.fetchAndProcess(f.ctx)
	assert.ErrorIs(t, err, ErrNothingToFetch)
}

func TestFetchAndProcessEvents(t *testing.T) {
	t.Parallel()

	f := setupProcessorTest(t)
	p := f.newProcessor(t, 10)

	authorId := f.addUser(t, "pe_author")
	sub1 := f.addUser(t, "pe_sub1")
	sub2 := f.addUser(t, "pe_sub2")
	require.NoError(t, f.userRepo.SubscribeUserTo(f.ctx, sub1, authorId))
	require.NoError(t, f.userRepo.SubscribeUserTo(f.ctx, sub2, authorId))

	post1 := newTestPost(authorId)
	post2 := newTestPost(authorId)
	f.enqueueEvent(t, database.OutboxMessageTypePostadded, post1)
	f.enqueueEvent(t, database.OutboxMessageTypePostadded, post2)
	f.enqueueEvent(t, database.OutboxMessageTypePostdeleted, post1)

	// sanity: all events are fresh, none processed yet
	unprocessed, processed := f.outboxCounts(t)
	assert.Equal(t, 3, unprocessed)
	assert.Equal(t, 0, processed)

	require.NoError(t, p.fetchAndProcess(f.ctx))

	// logic applied: post1 was added then deleted (so only post2 remains)
	for _, sub := range []uuid.UUID{sub1, sub2} {
		assert.Equal(t, []string{feedMember(post2)}, f.feedMembers(t, sub))
	}

	// all events are marked as processed
	unprocessed, processed = f.outboxCounts(t)
	assert.Equal(t, 0, unprocessed)
	assert.Equal(t, 3, processed)
}

func TestFetchAndProcessWithLimit(t *testing.T) {
	t.Parallel()

	const (
		eventCount = 5
		limit      = 3
	)

	f := setupProcessorTest(t)
	p := f.newProcessor(t, limit)

	authorId := f.addUser(t, "pl_author")
	sub := f.addUser(t, "pl_sub")
	require.NoError(t, f.userRepo.SubscribeUserTo(f.ctx, sub, authorId))

	posts := make([]*webModels.Post, 0, eventCount)
	for range eventCount {
		post := newTestPost(authorId)
		posts = append(posts, post)
		f.enqueueEvent(t, database.OutboxMessageTypePostadded, post)
	}

	// first fetch processes only up to the limit
	require.NoError(t, p.fetchAndProcess(f.ctx))
	unprocessed, processed := f.outboxCounts(t)
	assert.Equal(t, eventCount-limit, unprocessed)
	assert.Equal(t, limit, processed)

	// second fetch drains the rest
	require.NoError(t, p.fetchAndProcess(f.ctx))
	unprocessed, processed = f.outboxCounts(t)
	assert.Equal(t, 0, unprocessed)
	assert.Equal(t, eventCount, processed)

	// final state: every post is present in the subscriber's feed exactly once
	expected := make([]string, 0, eventCount)
	for _, post := range posts {
		expected = append(expected, feedMember(post))
	}
	assert.ElementsMatch(t, expected, f.feedMembers(t, sub))

	// nothing left to fetch
	assert.ErrorIs(t, p.fetchAndProcess(f.ctx), ErrNothingToFetch)
}
