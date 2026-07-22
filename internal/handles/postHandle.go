package handles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}

	cursorEncodedStr := base64.StdEncoding.EncodeToString(data)

	return cursorEncodedStr, nil
}

type PostsHandle struct {
	postService services.PostService
}

func NewPostsHandle(postService services.PostService) *PostsHandle {
	return &PostsHandle{
		postService: postService,
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
	cursorEncodedStr := ctx.DefaultQuery("cursor", "")
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if err != nil {
		ctx.Error(err)
		return
	}

	user_id, err := uuid.Parse(ctx.Param("user_id"))
	if err != nil {
		ctx.Error(err)
		return
	}

	var cursor *webModels.PostPaginationCursor = nil

	if err != nil {
		ctx.Error(err)
	}

	if cursorEncodedStr != "" {
		cursor, err = decodeCursor(cursorEncodedStr)
		if err != nil {
			ctx.Error(err)
			return
		}
	}

	posts, cursor, err := h.postService.GetPostsByAuthorId(ctx, user_id, cursor, limit)
	if err != nil {
		ctx.Error(err)
		return
	}

	encodedCursor, err := encodeCursor(cursor)
	if err != nil {
		ctx.Error(err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"posts":  posts,
		"cursor": encodedCursor,
	})
}

// @Summary			Send a post from the user
// @Description		An API route to send a post, requires an access token
// @Tags			posts
// @Accept			json
// @Produce			json
// @Security		Bearer
// @Param			post_data	body		webModels.CreatePostRequest	true	"Post payload"
// @Success			200			{object}	webModels.Post
// @Router			/post [post]
func (h *PostsHandle) SendPost(ctx *gin.Context) {
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

	var postInput webModels.CreatePostRequest

	if err := ctx.ShouldBindJSON(&postInput); err != nil {
		ctx.Error(err)
		return
	}

	post, err := h.postService.AddPost(ctx, postInput, userUuid)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, post)
}
