package handles

import (
	"net/http"

	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
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
	cursor, limit, err := getCursorAndLimit(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	userId, err := getUserId(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	posts, cursor, err := h.feedService.GetFromFeed(
		ctx.Request.Context(),
		userId,
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
