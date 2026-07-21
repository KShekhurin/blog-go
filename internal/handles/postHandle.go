package handles

import (
	"fmt"
	"net/http"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PostsHandle struct {
	postService services.PostService
}

func NewPostsHandle(postService services.PostService) *PostsHandle {
	return &PostsHandle{
		postService: postService,
	}
}

func (h *PostsHandle) GetPost(router *gin.RouterGroup) {

}

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
