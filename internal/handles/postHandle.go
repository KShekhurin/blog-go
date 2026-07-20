package handles

import (
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
)

type PostsHandle struct {
}

func NewPostsHandle() *PostsHandle {
	return &PostsHandle{}
}

func (h *PostsHandle) SendPost(ctx *gin.Context) {
	var postInput webModels.CreatePostRequest

	if err := ctx.ShouldBindJSON(&postInput); err != nil {
		ctx.Error(err)
		return
	}
}
