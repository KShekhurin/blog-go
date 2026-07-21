package webModels

import (
	"time"

	"github.com/google/uuid"
)

type PostPaginationCursor struct {
	LastId   uuid.UUID `json:"last_id" binding:"required"`
	LastTime time.Time `json:"last_time" binding:"required"`
}
