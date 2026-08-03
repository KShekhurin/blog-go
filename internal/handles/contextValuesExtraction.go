package handles

import (
	"errors"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	errUserIdMiddlewareIsEmpty = errors.New("user id was not set by middleware")
	errBadUserIdMiddleware     = errors.New("user id was not a UUID")
	errJtiMiddlewareIsEmpty    = errors.New("jti was not set by middleware")
	errBadJtiMiddleware        = errors.New("jti was not a UUID")
)

func getUserId(ctx *gin.Context) (uuid.UUID, error) {
	userId, exists := ctx.Get(middleware.UserIDKey)
	if !exists {
		return uuid.Nil, errUserIdMiddlewareIsEmpty
	}

	userUuid, ok := userId.(uuid.UUID)
	if !ok {
		return uuid.Nil, errBadUserIdMiddleware
	}

	return userUuid, nil
}

func getJti(ctx *gin.Context) (uuid.UUID, error) {
	jtiValue, exists := ctx.Get(middleware.RefreshJTIKey)
	if !exists {
		return uuid.Nil, errJtiMiddlewareIsEmpty
	}
	jti, ok := jtiValue.(uuid.UUID)
	if !ok {
		return uuid.Nil, errBadJtiMiddleware
	}

	return jti, nil
}
