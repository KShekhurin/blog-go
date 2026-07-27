package handles

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func decodeCursor(cursorEncodedStr string) (*webModels.PostPaginationCursor, error) {
	cursorDecodedStr, err := base64.StdEncoding.DecodeString(cursorEncodedStr)
	if err != nil {
		return nil, err
	}

	var cursor webModels.PostPaginationCursor
	if err := json.Unmarshal(cursorDecodedStr, &cursor); err != nil {
		return nil, err
	}

	validate := binding.Validator.Engine().(*validator.Validate)

	if err := validate.Struct(&cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

func encodeCursor(cursor *webModels.PostPaginationCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}

	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}

	cursorEncodedStr := base64.StdEncoding.EncodeToString(data)

	return cursorEncodedStr, nil
}

func getUserId(ctx *gin.Context) (uuid.UUID, error) {
	userId, exists := ctx.Get(middleware.UserIDKey)
	if !exists {
		return uuid.Nil, errors.New("user id not found")
	}

	userUuid, ok := userId.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("user id was not uuid")
	}

	return userUuid, nil
}

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
	userId, err := uuid.Parse(ctx.Param("userId"))
	if err != nil {
		ctx.Error(err)
		return
	}

	cursor, limit, err := getCursorAndLimit(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	posts, newCursor, err := h.postService.GetPostsByAuthorId(ctx, userId, cursor, limit)
	if err != nil {
		ctx.Error(err)
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
		ctx.Error(err)
		return
	}

	err = h.feedService.PushToFeeds(post)
	if err != nil {
		// We should log that the post didnt get to feeds, but it is present in db
		ctx.Error(err)
	}

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

	postId, err := uuid.Parse(ctx.Param("post_id"))
	if err != nil {
		ctx.Error(err)
		return
	}

	deletedPost, err := h.postService.RemovePostById(ctx.Request.Context(), postId, userId)
	if err != nil {
		ctx.Error(err)
		return
	}

	err = h.feedService.RemoveFromFeeds(deletedPost)
	if err != nil {
		ctx.Error(err)
	}

	ctx.JSON(http.StatusNoContent, gin.H{})
}
