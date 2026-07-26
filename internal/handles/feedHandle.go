package handles

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FeedHandle struct {
	feedService services.FeedService
}

func NewFeedHandle(feedService services.FeedService) *FeedHandle {
	return &FeedHandle{
		feedService: feedService,
	}
}

// @Summary			Get posts from the user's feed
// @Description		An API route to get the posts from user's feed, on consecutive requests pass the cursor
// @Tags			feed
// @Accept			json
// @Produce			json
// @Security Bearer
// @Param			cursor	query		string	false	"Pagination cursor"
// @Param			limit	query		int		false	"Post cnt limit"
// @Success			200		{object}	webModels.PostPaginationResponse
// @Router			/feed [get]
func (h *FeedHandle) GetPosts(ctx *gin.Context) {
	cursorEncodedStr := ctx.DefaultQuery("cursor", "")
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if err != nil {
		ctx.Error(err)
		return
	}

	var cursor *webModels.PostPaginationCursor = nil

	if cursorEncodedStr != "" {
		cursor, err = decodeCursor(cursorEncodedStr)
		if err != nil {
			ctx.Error(err)
			return
		}
	}

	userId, exists := ctx.Get(middleware.UserIDKey)
	if !exists {
		ctx.Error(fmt.Errorf("user id was not found"))
		return
	}

	userUuid, ok := userId.(uuid.UUID)
	if !ok {
		ctx.Error(fmt.Errorf("user id was not uuid"))
		return
	}

	posts, cursor, err := h.feedService.GetFromFeed(
		ctx.Request.Context(),
		userUuid,
		cursor,
		limit)

	if err != nil {
		ctx.Error(err)
		return
	}

	encodedCursor, err := encodeCursor(cursor)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, webModels.PostPaginationResponse{
		Posts:  posts,
		Cursor: encodedCursor,
	})
}
