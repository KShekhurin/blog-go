//go:build integration

package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/KShekhurin/blog-go/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

func setupPostRepoTest(t *testing.T) (PostRepository, *pgxpool.Pool, context.Context) {
	t.Helper()

	ctx := context.Background()
	pool := migrations.SetupPostgres(ctx, t)

	return NewPostRepository(pool), pool, ctx
}

// mustCreatePostAuthor inserts a user so posts satisfy the author_id foreign key.
// NOTE: adjust the column list/values to your actual users table schema
// (migrations/00001_create_user_table.sql).
func mustCreatePostAuthor(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	id := uuid.New()

	userRepo := NewUserRepository(database.New(pool))
	err := userRepo.AddUser(ctx, &database.User{
		ID:           id,
		Login:        fmt.Sprintf("%s", id)[:20],
		PasswordHash: "password",
	})

	require.NoError(t, err)

	return id
}

// utcNow truncates to microseconds because timestamptz has microsecond precision,
// which keeps time comparisons stable after a DB round trip.
func utcNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func buildTestPost(authorId uuid.UUID, createdAt time.Time) *webModels.Post {
	return &webModels.Post{
		Id:        uuid.New(),
		AuthorId:  authorId,
		Content:   "test content " + uuid.NewString(),
		CreatedAt: createdAt,
	}
}

func buildTestAttachment(postId uuid.UUID, displayOrder int) webModels.AttachedMedia {
	return webModels.AttachedMedia{
		Id:           uuid.New(),
		PostId:       postId,
		Type:         "image",
		MimeType:     "image/png",
		Url:          "https://example.com/media/" + uuid.NewString() + ".png",
		DisplayOrder: displayOrder,
	}
}

// mustAddPosts creates `count` posts with strictly increasing created_at.
// The returned slice is ordered oldest -> newest.
func mustAddPosts(t *testing.T, ctx context.Context, repo PostRepository, authorId uuid.UUID, count int) []*webModels.Post {
	t.Helper()

	base := utcNow()
	posts := make([]*webModels.Post, 0, count)
	for i := 0; i < count; i++ {
		post := buildTestPost(authorId, base.Add(time.Duration(i)*time.Minute))
		require.NoError(t, repo.AddPost(ctx, post))
		posts = append(posts, post)
	}
	return posts
}

func extractPostIDs(posts []webModels.Post) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.Id)
	}
	return ids
}

func extractAttachmentIDs(media []webModels.AttachedMedia) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(media))
	for _, m := range media {
		ids = append(ids, m.Id)
	}
	return ids
}

func requireOutboxEventCount(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	expectedType database.OutboxMessageType,
	expectedPostId uuid.UUID,
	expectedCount int,
) {
	t.Helper()

	var count int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox
		 WHERE message_type = $1
		   AND payload->>'id' = $2`,
		expectedType, expectedPostId.String(),
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, expectedCount, count,
		"unexpected number of outbox events of type %s for post %s", expectedType, expectedPostId)
}

// requireOutboxEvent asserts that the outbox contains exactly one event of the
// given type whose payload references the given post id.
func requireOutboxEvent(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	expectedType database.OutboxMessageType,
	expectedPostId uuid.UUID,
) {
	t.Helper()

	requireOutboxEventCount(t, ctx, pool, expectedType, expectedPostId, 1)
}

// --- AddPost ---

func TestAddPost(t *testing.T) {
	t.Run("successfully adds post with zero attachments", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())

		require.NoError(t, repo.AddPost(ctx, post))

		stored, err := repo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Equal(t, post.Id, stored.Id)
		require.Equal(t, post.AuthorId, stored.AuthorId)
		require.Equal(t, post.Content, stored.Content)
		require.True(t, post.CreatedAt.Equal(stored.CreatedAt))
		require.Nil(t, stored.ReplyTo)
		require.Nil(t, stored.DeletedAt)
		require.Empty(t, stored.Attached)

		requireOutboxEvent(t, ctx, pool, database.OutboxMessageTypePostadded, post.Id)
	})

	t.Run("successfully adds post with several attachments", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())
		post.Attached = []webModels.AttachedMedia{
			buildTestAttachment(post.Id, 0),
			buildTestAttachment(post.Id, 1),
			buildTestAttachment(post.Id, 2),
		}

		require.NoError(t, repo.AddPost(ctx, post))

		stored, err := repo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Len(t, stored.Attached, len(post.Attached))
		require.ElementsMatch(t, extractAttachmentIDs(post.Attached), extractAttachmentIDs(stored.Attached))

		requireOutboxEvent(t, ctx, pool, database.OutboxMessageTypePostadded, post.Id)
	})

	t.Run("successfully adds post that is a reply", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		parent := buildTestPost(authorId, utcNow())
		require.NoError(t, repo.AddPost(ctx, parent))

		reply := buildTestPost(authorId, utcNow())
		reply.ReplyTo = &parent.Id

		require.NoError(t, repo.AddPost(ctx, reply))

		stored, err := repo.GetPostById(ctx, reply.Id)
		require.NoError(t, err)
		require.NotNil(t, stored.ReplyTo)
		require.Equal(t, parent.Id, *stored.ReplyTo)

		requireOutboxEvent(t, ctx, pool, database.OutboxMessageTypePostadded, reply.Id)
	})

	t.Run("returns error on unique violation", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())
		require.NoError(t, repo.AddPost(ctx, post))

		duplicate := buildTestPost(authorId, utcNow())
		duplicate.Id = post.Id

		err := repo.AddPost(ctx, duplicate)
		require.Error(t, err)
		require.ErrorIs(t, err, errs.ErrAlreadyExists)

		requireOutboxEvent(t, ctx, pool, database.OutboxMessageTypePostadded, post.Id)
	})
}

// --- RemovePostById ---

func TestRemovePost(t *testing.T) {
	t.Run("successfully sets deleted_at", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())
		require.NoError(t, repo.AddPost(ctx, post))

		removeAt := utcNow()
		require.NoError(t, repo.RemovePost(ctx, post, removeAt))

		stored, err := repo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.NotNil(t, stored.DeletedAt)
		require.True(t, removeAt.Equal(*stored.DeletedAt),
			"expected deleted_at %v, got %v", removeAt, *stored.DeletedAt)

		requireOutboxEvent(t, ctx, pool, database.OutboxMessageTypePostdeleted, post.Id)
	})

	t.Run("returns error when post does not exist", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)

		missing := buildTestPost(uuid.New(), utcNow())

		err := repo.RemovePost(ctx, missing, utcNow())
		require.Error(t, err)
		require.ErrorIs(t, err, errs.ErrNotFound)

		requireOutboxEventCount(t, ctx, pool, database.OutboxMessageTypePostdeleted, missing.Id, 0)
	})
}

// --- GetPostById ---

func TestGetPostById(t *testing.T) {
	t.Run("successfully fetches post without attachments", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())
		require.NoError(t, repo.AddPost(ctx, post))

		stored, err := repo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Equal(t, post.Id, stored.Id)
		require.Equal(t, post.AuthorId, stored.AuthorId)
		require.Equal(t, post.Content, stored.Content)
		require.True(t, post.CreatedAt.Equal(stored.CreatedAt))
		require.Empty(t, stored.Attached)
	})

	t.Run("successfully fetches post with attachments", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		post := buildTestPost(authorId, utcNow())
		post.Attached = []webModels.AttachedMedia{
			buildTestAttachment(post.Id, 0),
			buildTestAttachment(post.Id, 1),
		}
		require.NoError(t, repo.AddPost(ctx, post))

		stored, err := repo.GetPostById(ctx, post.Id)
		require.NoError(t, err)
		require.Equal(t, post.Id, stored.Id)
		require.Len(t, stored.Attached, len(post.Attached))

		byId := make(map[uuid.UUID]webModels.AttachedMedia, len(stored.Attached))
		for _, m := range stored.Attached {
			byId[m.Id] = m
		}
		for _, want := range post.Attached {
			got, ok := byId[want.Id]
			require.True(t, ok, "attachment %s not found", want.Id)
			require.Equal(t, want.PostId, got.PostId)
			require.Equal(t, want.Type, got.Type)
			require.Equal(t, want.MimeType, got.MimeType)
			require.Equal(t, want.Url, got.Url)
			require.Equal(t, want.DisplayOrder, got.DisplayOrder)
		}
	})

	t.Run("returns error when post not found", func(t *testing.T) {
		t.Parallel()
		repo, _, ctx := setupPostRepoTest(t)

		_, err := repo.GetPostById(ctx, uuid.New())
		require.Error(t, err)
		require.ErrorIs(t, err, errs.ErrNotFound)
	})
}

// --- GetPostsWithIds ---

func TestGetPostsWithIds(t *testing.T) {
	t.Run("finds all posts with listed ids", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 3)
		ids := []uuid.UUID{created[0].Id, created[1].Id, created[2].Id}

		found, err := repo.GetPostsWithIds(ctx, ids)
		require.NoError(t, err)
		require.Len(t, found, 3)
		require.ElementsMatch(t, ids, extractPostIDs(found))
	})

	t.Run("finds posts partially when some ids do not exist", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 2)

		found, err := repo.GetPostsWithIds(ctx, []uuid.UUID{
			created[0].Id, created[1].Id, uuid.New(), uuid.New(),
		})
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.ElementsMatch(t, []uuid.UUID{created[0].Id, created[1].Id}, extractPostIDs(found))
	})

	t.Run("does not return deleted posts", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 3)
		deleted := created[1]
		require.NoError(t, repo.RemovePost(ctx, deleted, utcNow()))

		found, err := repo.GetPostsWithIds(ctx, []uuid.UUID{created[0].Id, deleted.Id, created[2].Id})
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.ElementsMatch(t, []uuid.UUID{created[0].Id, created[2].Id}, extractPostIDs(found))
	})
}

// --- GetPostsByAuthorId ---

func TestGetPostsByAuthorId(t *testing.T) {
	t.Run("no cursor: returns first page and cursor pointing at the last post", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 5)

		page, cursor, err := repo.GetPostsByAuthorId(ctx, authorId, nil, 3)
		require.NoError(t, err)
		require.Len(t, page, 3)
		// newest first
		require.Equal(t,
			[]uuid.UUID{created[4].Id, created[3].Id, created[2].Id},
			extractPostIDs(page),
		)

		require.NotNil(t, cursor)
		require.Equal(t, created[2].Id, cursor.LastId)
		require.True(t, created[2].CreatedAt.Equal(cursor.LastTime))
	})

	t.Run("with cursor: fetches posts after the cursor position", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 5)

		page1, cursor, err := repo.GetPostsByAuthorId(ctx, authorId, nil, 3)
		require.NoError(t, err)
		require.Len(t, page1, 3)
		require.NotNil(t, cursor)

		page2, cursor2, err := repo.GetPostsByAuthorId(ctx, authorId, cursor, 3)
		require.NoError(t, err)
		require.Len(t, page2, 2)
		require.Equal(t,
			[]uuid.UUID{created[1].Id, created[0].Id},
			extractPostIDs(page2),
		)

		require.NotNil(t, cursor2)
		require.Equal(t, created[0].Id, cursor2.LastId)
	})

	t.Run("does not fetch deleted posts", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		created := mustAddPosts(t, ctx, repo, authorId, 4)
		deleted := created[2]
		require.NoError(t, repo.RemovePost(ctx, deleted, utcNow()))

		page, cursor, err := repo.GetPostsByAuthorId(ctx, authorId, nil, 10)
		require.NoError(t, err)
		require.Len(t, page, 3)
		require.Equal(t,
			[]uuid.UUID{created[3].Id, created[1].Id, created[0].Id},
			extractPostIDs(page),
		)
		for _, post := range page {
			require.Nil(t, post.DeletedAt)
		}
		require.NotNil(t, cursor)
	})

	t.Run("cursor at last post: returns zero posts and empty cursor", func(t *testing.T) {
		t.Parallel()
		repo, pool, ctx := setupPostRepoTest(t)
		authorId := mustCreatePostAuthor(t, ctx, pool)

		mustAddPosts(t, ctx, repo, authorId, 2)

		page1, cursor, err := repo.GetPostsByAuthorId(ctx, authorId, nil, 10)
		require.NoError(t, err)
		require.Len(t, page1, 2)
		require.NotNil(t, cursor)

		page2, cursor2, err := repo.GetPostsByAuthorId(ctx, authorId, cursor, 10)
		require.NoError(t, err)
		require.Empty(t, page2)
		require.Nil(t, cursor2)
	})
}
