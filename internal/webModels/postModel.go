package webModels

import (
	"time"

	"github.com/google/uuid"
)

// Out

type AttachedMedia struct {
	Id           uuid.UUID `json:"id"`
	PostId       uuid.UUID `json:"post_id"`
	Type         string    `json:"type"`
	MimeType     string    `json:"mime_type"`
	Url          string    `json:"url"`
	DisplayOrder int       `json:"display_order"`
}

type Post struct {
	Id        uuid.UUID       `json:"id"`
	AuthorId  uuid.UUID       `json:"author_id"`
	ReplyTo   *uuid.UUID      `json:"reply_to,omitempty"`
	Content   string          `json:"content"`
	CreatedAt time.Time       `json:"created_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
	Attached  []AttachedMedia `json:"attached"`
}

// In

type CreateAttachedMedia struct {
	Type         string `json:"type" binding:"required"`
	MimeType     string `json:"mime_type" binding:"required"`
	Url          string `json:"url" binding:"required,url"`
	DisplayOrder int    `json:"display_order" binding:"gte=0"`
}

type CreatePostRequest struct {
	ReplyTo  *uuid.UUID            `json:"reply_to" binding:"omitempty"`
	Content  string                `json:"content" binding:"required"`
	Attached []CreateAttachedMedia `json:"attached" binding:"omitempty,dive"`
}
