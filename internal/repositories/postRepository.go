package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func toPostParams(post *webModels.Post) database.AddPostParams {
	replyTo := uuid.NullUUID{Valid: false}

	if post.ReplyTo != nil {
		replyTo.UUID = *post.ReplyTo
		replyTo.Valid = true
	}

	deletedAt := pgtype.Timestamptz{}

	if post.DeletedAt != nil {
		deletedAt.Time = *post.DeletedAt
		deletedAt.Valid = true
	}

	return database.AddPostParams{
		ID:       post.Id,
		AuthorID: post.AuthorId,
		ReplyTo:  replyTo,
		Content:  post.Content,
		CreatedAt: pgtype.Timestamptz{
			Time:  post.CreatedAt,
			Valid: true,
		},
		DeletedAt: deletedAt,
	}
}

func toAddPostAttachmentsParams(attachments []webModels.AttachedMedia) []database.AddPostMediaParams {
	addPostMediaParams := make([]database.AddPostMediaParams, 0, len(attachments))

	for _, attachment := range attachments {
		addPostMediaParams = append(addPostMediaParams, database.AddPostMediaParams{
			ID:           attachment.Id,
			PostID:       attachment.PostId,
			Type:         database.MediaType(attachment.Type),
			MimeType:     attachment.MimeType,
			Url:          attachment.Url,
			DisplayOrder: int32(attachment.DisplayOrder),
		})
	}

	return addPostMediaParams
}

type PostRepository interface {
	AddPost(ctx context.Context, post *webModels.Post) error
	RemovePost(ctx context.Context, post *webModels.Post, removeAt time.Time) error
	GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error)
	GetPostsWithIds(ctx context.Context, ids []uuid.UUID) ([]webModels.Post, error)
	GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error)
}

type postRepository struct {
	pool  *pgxpool.Pool
	query *database.Queries
}

func NewPostRepository(pool *pgxpool.Pool) PostRepository {
	return &postRepository{
		pool:  pool,
		query: database.New(pool),
	}
}

func (r *postRepository) RemovePost(ctx context.Context, post *webModels.Post, removeAt time.Time) error {
	err := ExecTransaction(ctx, r.pool,
		func(q *database.Queries) error {
			cnt, err := r.query.DeletePost(ctx, database.DeletePostParams{
				ID: post.Id,
				DeletedAt: pgtype.Timestamptz{
					Time:  removeAt,
					Valid: true,
				},
			})

			if err != nil {
				return err
			}

			if cnt == 0 {
				return &errs.NotFoundError{ID: post.Id.String(), Resource: "RemovePostById"}
			}

			err = q.AddToOutbox(ctx, database.AddToOutboxParams{
				MessageType: database.OutboxMessageTypePostadded,
				Payload:     post,
			})
			if err != nil {
				return err
			}

			return nil
		})

	return err
}

func (r *postRepository) AddPost(ctx context.Context, post *webModels.Post) error {
	err := ExecTransaction(ctx, r.pool,
		func(q *database.Queries) error {
			addPostParams := toPostParams(post)
			addAttachmentsParams := toAddPostAttachmentsParams(post.Attached)

			err := q.AddPost(ctx, addPostParams)
			if err != nil {
				return err
			}

			for _, attachmentParam := range addAttachmentsParams {
				err := q.AddPostMedia(ctx, attachmentParam)
				if err != nil {
					return err
				}
			}

			err = q.AddToOutbox(ctx, database.AddToOutboxParams{
				MessageType: database.OutboxMessageTypePostadded,
				Payload:     post,
			})
			if err != nil {
				return err
			}

			return nil
		})

	if err != nil {
		if IsUniqueViolation(err) {
			return fmt.Errorf("post with such id already exists: %w", errs.ErrAlreadyExists)
		}
		return fmt.Errorf("failed to add post: %w", err)
	}

	return nil
}

func toAttachment(postMediaData *database.PostMedium) webModels.AttachedMedia {
	return webModels.AttachedMedia{
		Id:           postMediaData.ID,
		PostId:       postMediaData.PostID,
		Type:         string(postMediaData.Type),
		MimeType:     postMediaData.MimeType,
		Url:          postMediaData.Url,
		DisplayOrder: int(postMediaData.DisplayOrder),
	}
}

func toAttachments(postsMediaData []database.PostMedium) []webModels.AttachedMedia {
	attachments := make([]webModels.AttachedMedia, 0, len(postsMediaData))

	for i := range postsMediaData {
		attachments = append(attachments, toAttachment(&postsMediaData[i]))
	}

	return attachments
}

func toPost(postData *database.Post, attachments []webModels.AttachedMedia) webModels.Post {
	var replyTo *uuid.UUID = nil
	if postData.ReplyTo.Valid {
		replyTo = &postData.ReplyTo.UUID
	}

	var deletedAt *time.Time = nil
	if postData.DeletedAt.Valid {
		deletedAt = &postData.DeletedAt.Time
	}

	return webModels.Post{
		Id:        postData.ID,
		AuthorId:  postData.AuthorID,
		Content:   postData.Content,
		ReplyTo:   replyTo,
		CreatedAt: postData.CreatedAt.Time,
		DeletedAt: deletedAt,
		Attached:  attachments,
	}
}

func (r *postRepository) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	postData, err := r.query.FindPostById(ctx, id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &errs.NotFoundError{ID: id.String(), Resource: "GetPostById"}
		}
		return nil, fmt.Errorf("failed to find post by id: %w", err)
	}

	postAttachments, err := r.query.FindLinkedPostMedia(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find linked post media: %w", err)
	}

	attachments := toAttachments(postAttachments)
	post := toPost(&postData, attachments)
	return &post, nil
}

func gatherPostIDs(postsData []database.Post) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(postsData))

	for _, postData := range postsData {
		ids = append(ids, postData.ID)
	}

	return ids
}

func mapPostsMediaByPostId(postsMedia []database.PostMedium) map[uuid.UUID][]webModels.AttachedMedia {
	uuidToAttachmentsSlice := make(map[uuid.UUID][]webModels.AttachedMedia)

	for _, postMediaData := range postsMedia {
		uuidToAttachmentsSlice[postMediaData.PostID] = append(
			uuidToAttachmentsSlice[postMediaData.PostID],
			toAttachment(&postMediaData))
	}

	return uuidToAttachmentsSlice
}

func makeNewCursor(posts []webModels.Post) *webModels.PostPaginationCursor {
	lastPost := posts[len(posts)-1]

	return &webModels.PostPaginationCursor{
		LastId:   lastPost.Id,
		LastTime: lastPost.CreatedAt,
	}
}

func (r *postRepository) GetPostsByAuthorId(
	ctx context.Context,
	authorId uuid.UUID,
	cursor *webModels.PostPaginationCursor,
	limit int,
) ([]webModels.Post, *webModels.PostPaginationCursor, error) {

	var postsData []database.Post
	var err error

	if cursor == nil {
		postsData, err = r.query.FindUserPosts(ctx,
			database.FindUserPostsParams{
				AuthorID: authorId,
				Limit:    int32(limit),
			})
	} else {
		postsData, err = r.query.FindUserPostsWithCursor(ctx,
			database.FindUserPostsWithCursorParams{
				AuthorID: authorId,
				Limit:    int32(limit),
				LastCreatedAt: pgtype.Timestamptz{
					Time:  cursor.LastTime,
					Valid: true,
				},
				LastID: cursor.LastId,
			})
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to find posts: %w", err)
	}

	if len(postsData) == 0 {
		return []webModels.Post{}, nil, nil
	}

	postIds := gatherPostIDs(postsData)

	attachedMedia, err := r.query.FindPostsMedias(ctx, postIds)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to find attached posts' medias: %w", err)
	}

	uuidToAttachments := mapPostsMediaByPostId(attachedMedia)

	posts := make([]webModels.Post, 0, len(postsData))

	for _, postData := range postsData {
		posts = append(
			posts,
			toPost(&postData, uuidToAttachments[postData.ID]))
	}

	return posts, makeNewCursor(posts), nil
}

func (r *postRepository) GetPostsWithIds(ctx context.Context, ids []uuid.UUID) ([]webModels.Post, error) {
	postsData, err := r.query.FindPostsByIds(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to find posts: %w", err)
	}

	if len(postsData) == 0 {
		return []webModels.Post{}, nil
	}

	postIds := gatherPostIDs(postsData)

	attachedMedia, err := r.query.FindPostsMedias(ctx, postIds)

	if err != nil {
		return nil, fmt.Errorf("failed to find attached posts' medias: %w", err)
	}

	uuidToAttachments := mapPostsMediaByPostId(attachedMedia)

	posts := make([]webModels.Post, 0, len(postsData))

	for _, postData := range postsData {
		posts = append(
			posts,
			toPost(&postData, uuidToAttachments[postData.ID]))
	}

	return posts, nil
}
