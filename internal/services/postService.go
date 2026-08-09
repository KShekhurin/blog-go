package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

var (
	ErrorPostWasDeleted   = errors.New("post was deleted")
	ErrorPostDoesNotExist = errors.New("post does not exist")
)

type PostService interface {
	AddPost(ctx context.Context, request webModels.CreatePostRequest, authorId uuid.UUID) (*webModels.Post, error)
	GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error)
	RemovePostById(ctx context.Context, id uuid.UUID, userId uuid.UUID) (*webModels.Post, error)
	GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error)
}

type postService struct {
	postRepo repositories.PostRepository
}

func NewPostService(postRepo repositories.PostRepository) PostService {
	return &postService{
		postRepo: postRepo,
	}
}

func (s *postService) RemovePostById(ctx context.Context, postId uuid.UUID, userId uuid.UUID) (*webModels.Post, error) {
	post, err := s.postRepo.GetPostById(ctx, postId)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return nil, ErrorPostDoesNotExist
		}
		return nil, fmt.Errorf("failed to get post by id: %w", err)
	}

	if post.AuthorId != userId {
		return nil, ErrorUnauthorized
	}

	deletedAt := time.Now()
	post.DeletedAt = &deletedAt

	err = s.postRepo.RemovePostById(ctx, postId, deletedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to remove post by id: %w", err)
	}

	return post, nil
}

func toAttachedMedia(request []webModels.CreateAttachedMedia, postId uuid.UUID) []webModels.AttachedMedia {
	attachedMedia := make([]webModels.AttachedMedia, 0, len(request))

	for _, mediaRequest := range request {
		attachedMedia = append(attachedMedia, webModels.AttachedMedia{
			Id:           uuid.New(),
			PostId:       postId,
			Type:         mediaRequest.Type,
			MimeType:     mediaRequest.MimeType,
			Url:          mediaRequest.Url,
			DisplayOrder: mediaRequest.DisplayOrder,
		})
	}

	return attachedMedia
}

func toPost(request *webModels.CreatePostRequest, authorId uuid.UUID) *webModels.Post {
	creationTime := time.Now()
	postId := uuid.New()

	return &webModels.Post{
		Id:        postId,
		AuthorId:  authorId,
		ReplyTo:   request.ReplyTo,
		Content:   request.Content,
		CreatedAt: creationTime,
		Attached:  toAttachedMedia(request.Attached, postId),
	}
}

func (s *postService) GetPostsByAuthorId(ctx context.Context, authorId uuid.UUID, cursor *webModels.PostPaginationCursor, limit int) ([]webModels.Post, *webModels.PostPaginationCursor, error) {
	posts, cursor, err := s.postRepo.GetPostsByAuthorId(
		ctx,
		authorId,
		cursor,
		limit,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("could not gather posts by author id %s: %w", authorId, err)
	}

	return posts, cursor, nil
}

func (s *postService) AddPost(ctx context.Context, request webModels.CreatePostRequest, authorId uuid.UUID) (*webModels.Post, error) {
	post := toPost(&request, authorId)

	err := s.postRepo.AddPost(ctx, post)

	if err != nil {
		return nil, fmt.Errorf("could not add post: %w", err)
	}

	return post, nil
}

func (s *postService) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	post, err := s.postRepo.GetPostById(ctx, id)

	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return nil, ErrorPostDoesNotExist
		}
		return nil, fmt.Errorf("could not get post by id %s: %w", id, err)
	}

	if post.DeletedAt != nil {
		return nil, ErrorPostWasDeleted
	}

	return post, nil
}
