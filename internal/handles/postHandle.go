package handles

import (
	"net/http"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	errUserIdPathIsEmpty = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "user_id_is_empty",
		Message:  "user id in path cannot be empty",
	}
	errBadUserIdPath = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "user_id_is_invalid",
		Message:  "user id in path must be a valid UUID",
	}
	errPostIdPathIsEmpty = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "post_id_is_empty",
		Message:  "Post id in path cannot be empty",
	}
	errBadPostIdPath = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "post_id_is_invalid",
		Message:  "Post id in path must be a valid UUID",
	}
)

type PostsHandle struct {
	postService services.PostService
	feedService services.FeedService
}

func NewPostsHandle(postService services.PostService, feedService services.FeedService) *PostsHandle {
	return &PostsHandle{
		postService: postService,
		feedService: feedService,
	}
}

// @Summary			Get posts from the user
// @Description		An API route to get the user's post, on consecutive requests pass the cursor
// @Tags			posts
// @Accept			json
// @Produce			json
// @Param			user_id	path		string	true	"User ID"	Format(uuid)	example:"550e8400-e29b-41d4-a716-446655440000"
// @Param			cursor	query		string	false	"Pagination cursor"
// @Param			limit	query		int		false	"Post cnt limit"
// @Success			200		{object}	webModels.PostPaginationResponse
// @Router			/user/{user_id}/posts [get]
func (h *PostsHandle) GetPostsByUserId(ctx *gin.Context) {
	userIdStr := ctx.Param("user_id")
	if userIdStr == "" {
		ctx.Error(&errUserIdPathIsEmpty)
		return
	}

	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		ctx.Error(&errBadUserIdPath)
		return
	}

	cursor, limit, err := getCursorAndLimit(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	posts, newCursor, err := h.postService.GetPostsByAuthorId(ctx, userId, cursor, limit)
	if err != nil {
		ctx.Error(processPostServiceErrors(err))
		return
	}

	encodedNewCursor, err := encodeCursor(newCursor)
	if err != nil {
		ctx.Error(err)
	}

	ctx.JSON(http.StatusOK, webModels.PostPaginationResponse{
		Posts:  posts,
		Cursor: encodedNewCursor,
	})
}

// @Summary			Send a post from the user
// @Description		An API route to send a post, requires an access token
// @Description 	Also pushes the post to user feeds in background
// @Tags			posts
// @Accept			json
// @Produce			json
// @Security		Bearer
// @Param			post_data	body		webModels.CreatePostRequest	true	"Post payload"
// @Success			200			{object}	webModels.Post
// @Router			/post [post]
func (h *PostsHandle) SendPost(ctx *gin.Context) {
	userId, err := getUserId(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var postInput webModels.CreatePostRequest

	if err := ctx.ShouldBindJSON(&postInput); err != nil {
		ctx.Error(err)
		return
	}

	post, err := h.postService.AddPost(ctx, postInput, userId)
	if err != nil {
		ctx.Error(processPostServiceErrors(err))
		return
	}

	//Fire and forget
	h.feedService.PushToFeeds(post)

	ctx.JSON(http.StatusOK, post)
}

// @Summary			Delete post
// @Description		An API route to delete a post, requires an access token
// @Description 	The post gets deleted_at field and is not avaliable for getting via api
// @Description		Also removes post from users feeds
// @Tags			posts
// @Accept			json
// @Produce			json
// @Security		Bearer
// @Param 			post_id		path		string	true	"Post ID"	Format(uuid)	example:"550e8400-e29b-41d4-a716-446655440000"
// @Success			204			"No Content"
// @Router			/post/{post_id} [delete]
func (h *PostsHandle) DeletePostById(ctx *gin.Context) {
	userId, err := getUserId(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	postIdStr := ctx.Param("post_id")
	if postIdStr == "" {
		ctx.Error(&errPostIdPathIsEmpty)
		return
	}

	postId, err := uuid.Parse(postIdStr)
	if err != nil {
		ctx.Error(&errBadPostIdPath)
		return
	}

	deletedPost, err := h.postService.RemovePostById(ctx.Request.Context(), postId, userId)
	if err != nil {
		ctx.Error(processPostServiceErrors(err))
		return
	}

	//Fire and forget.
	h.feedService.RemoveFromFeeds(deletedPost)

	ctx.JSON(http.StatusNoContent, gin.H{})
}
