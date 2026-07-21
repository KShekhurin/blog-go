package services

import (
	"context"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
)

var (
	ErrorPostWasDeleted = fmt.Errorf("post was deleted")
)

type PostService interface {
	AddPost(ctx context.Context, request webModels.CreatePostRequest, authorId uuid.UUID) (*webModels.Post, error)
	GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error)
}

type postService struct {
	postRepo repositories.PostRepository
}

func NewPostService(postRepo repositories.PostRepository) PostService {
	return &postService{
		postRepo: postRepo,
	}
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

func (s postService) AddPost(ctx context.Context, request webModels.CreatePostRequest, authorId uuid.UUID) (*webModels.Post, error) {
	post := toPost(&request, authorId)

	err := s.postRepo.AddPost(ctx, post)

	if err != nil {
		return nil, err
	}

	return post, nil
}

func (s postService) GetPostById(ctx context.Context, id uuid.UUID) (*webModels.Post, error) {
	post, err := s.postRepo.GetPostById(ctx, id)

	if err != nil {
		return nil, err
	}

	if post.DeletedAt != nil {
		return nil, ErrorPostWasDeleted
	}

	return post, nil
}
