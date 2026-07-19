package handles

import (
	"net/http"

	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userService services.UserService
	authService services.AuthService
}

func NewAuthHandler(userService services.UserService, authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		userService: userService,
		authService: authService,
	}
}

func (h *AuthHandler) Register(ctx *gin.Context) {
	var registerInfo webModels.UserRegisterInfo

	if err := ctx.ShouldBindBodyWithJSON(&registerInfo); err != nil {
		ctx.Error(err)
		return
	}

	id, err := h.userService.CreateUser(ctx.Request.Context(), &registerInfo)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var userLoginInfo webModels.UserLoginInfo

	if err := ctx.ShouldBindBodyWithJSON(&userLoginInfo); err != nil {
		ctx.Error(err)
		return
	}

	token, err := h.authService.SignJWT(ctx.Request.Context(), &userLoginInfo)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": token})
}
