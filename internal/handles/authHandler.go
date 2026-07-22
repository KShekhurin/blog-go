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

// @Summary		Register a new user
// @Description	An API route to register a new user, on success returns access/refresh jwt pair
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		user_data	body		webModels.UserRegisterInfo	true	"Registration payload"
// @Success		200			{object}	webModels.TokenPair
// @Router		/auth/register [post]
func (h *AuthHandler) Register(ctx *gin.Context) {
	var registerInfo webModels.UserRegisterInfo

	if err := ctx.ShouldBindBodyWithJSON(&registerInfo); err != nil {
		ctx.Error(err)
		return
	}

	user, err := h.userService.CreateUser(ctx.Request.Context(), &registerInfo)

	if err != nil {
		ctx.Error(err)
		return
	}

	tokens, err := h.authService.SignJWT(ctx.Request.Context(), user)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusCreated, tokens)
}

// @Summary		Login with existing user
// @Description	An API route to login an user, on success returns access/refresh jwt pair
// @Description Ether login or email is required
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		user_data	body		webModels.UserLoginInfo	true	"Login payload"
// @Success		200			{object}	webModels.TokenPair
// @Router		/auth/login [post]
func (h *AuthHandler) Login(ctx *gin.Context) {
	var userLoginInfo webModels.UserLoginInfo

	if err := ctx.ShouldBindBodyWithJSON(&userLoginInfo); err != nil {
		ctx.Error(err)
		return
	}

	user, err := h.authService.AuthenticateUser(ctx.Request.Context(), &userLoginInfo)
	if err != nil {
		ctx.Error(err)
		return
	}

	tokens, err := h.authService.SignJWT(ctx.Request.Context(), user)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, tokens)
}
