package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func toPostParams(post *webModels.Post) database.AddPostParams {
	replyTo := uuid.NullUUID{}

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
	GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error)
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

			return nil
		})

	if err != nil {
		if IsUniqueViolation(err) {
			return ErrorUniqueViolation
		}
		return fmt.Errorf("failed to add post: %w", err)
	}

	return nil
}

func (r *postRepository) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	postData, err := r.query.FindPostById(ctx, id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user was not found: %w", ErrorDoesNotExist)
		}
		return nil, fmt.Errorf("failed to find post by id: %w", err)
	}

	postAttachments, err := r.query.FindLinkedPostMedia(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find liked post media: %w", err)
	}

	var replyTo *uuid.UUID = nil
	if postData.ReplyTo.Valid {
		replyTo = &postData.ReplyTo.UUID
	}

	var deletedAt *time.Time = nil
	if postData.DeletedAt.Valid {
		deletedAt = &postData.DeletedAt.Time
	}

	attachments := make([]webModels.AttachedMedia, 0, len(postAttachments))

	for _, postAttachmentData := range postAttachments {
		attachments = append(attachments, webModels.AttachedMedia{
			Id:           postAttachmentData.ID,
			PostId:       postAttachmentData.PostID,
			Type:         string(postAttachmentData.Type),
			MimeType:     postAttachmentData.MimeType,
			Url:          postAttachmentData.Url,
			DisplayOrder: int(postAttachmentData.DisplayOrder),
		})
	}

	return &webModels.Post{
		Id:        postData.ID,
		AuthorId:  postData.AuthorID,
		Content:   postData.Content,
		ReplyTo:   replyTo,
		CreatedAt: postData.CreatedAt.Time,
		DeletedAt: deletedAt,
		Attached:  attachments,
	}, nil
}
