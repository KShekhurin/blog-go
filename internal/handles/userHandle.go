package handles

import (
	"fmt"
	"net/http"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	errMissingAuthorId = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "missing_author_id",
		Message:  "author_id is required",
	}
	errWrongAuthorIdFormat = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "wrong_author_id_format",
		Message:  "author_id is invalid",
	}
)

type UserHandle struct {
	userService services.UserService
}

func NewUserHandler(userService services.UserService) *UserHandle {
	return &UserHandle{
		userService: userService,
	}
}

// @Summary		Subscribe the user to the author
// @Description	An API route to subscribe user to the author with the specified ID
// @Description User's ID is provided in JWT token
// @Tags		user
// @Security 	Bearer
// @Param		author_id	path		string	true	"Author ID"	Format(uuid)	example:"550e8400-e29b-41d4-a716-446655440000"
// @Success		204    "No Content"
// @Router		/user/{author_id}/subs [post]
func (h *UserHandle) SubscribeTo(ctx *gin.Context) {
	userId, exists := ctx.Get(middleware.UserIDKey)
	if !exists {
		ctx.Error(fmt.Errorf("user id was not found"))
		return
	}

	idString := ctx.Param("author_id")
	if idString == "" {
		ctx.Error(&errMissingAuthorId)
		return
	}

	authorId, err := uuid.Parse(idString)
	if err != nil {
		ctx.Error(&errWrongAuthorIdFormat)
		return
	}

	err = h.userService.SubscribeTo(ctx.Request.Context(), userId.(uuid.UUID), authorId)
	if err != nil {
		ctx.Error(processUserServiceErrors(err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

// @Summary		Unsubscribe the user from the author
// @Description	An API route to unsubscribe user with the author with the specified ID
// @Description User's ID is provided in JWT token
// @Tags		user
// @Security 	Bearer
// @Param		author_id	path		string	true	"Author ID"	Format(uuid)	example:"550e8400-e29b-41d4-a716-446655440000"
// @Success		204    "No Content"
// @Router		/user/{author_id}/subs [delete]
func (h *UserHandle) UnsubscribeFrom(ctx *gin.Context) {
	userId, exists := ctx.Get(middleware.UserIDKey)
	if !exists {
		ctx.Error(fmt.Errorf("user id was not found"))
		return
	}

	idString := ctx.Param("author_id")
	if idString == "" {
		ctx.Error(&errMissingAuthorId)
		return
	}

	authorId, err := uuid.Parse(idString)
	if err != nil {
		ctx.Error(&errWrongAuthorIdFormat)
		return
	}

	err = h.userService.UnsubscribeFrom(ctx.Request.Context(), userId.(uuid.UUID), authorId)
	if err != nil {
		ctx.Error(processUserServiceErrors(err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}
