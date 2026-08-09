package handles

import (
	"errors"
	"net/http"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/services"
)

func processUserServiceErrors(err error) error {
	switch {
	case errors.Is(err, services.ErrNotSubscribed):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusBadRequest,
			ErrorTag: "not_subscribed",
			Message:  "User is not subscribed",
		}
	case errors.Is(err, services.ErrUserNotFound):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusNotFound,
			ErrorTag: "user_not_found",
			Message:  "User with such credentials does not exist",
		}

	case errors.Is(err, services.ErrAlreadySubscribed):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusConflict,
			ErrorTag: "user_already_subscribed",
			Message:  "User is already subscribed",
		}
	case errors.Is(err, services.ErrUserAlreadyExists):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusConflict,
			ErrorTag: "user_already_exists",
			Message:  "User with such credentials already exists",
		}
	default:
		return err
	}
}

func processAuthServiceErrors(err error) error {
	switch {
	case errors.Is(err, services.ErrInvalidCredentials):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusBadRequest,
			ErrorTag: "invalid_credentials",
			Message:  "Invalid Credentials",
		}
	case errors.Is(err, services.ErrTokenBanned):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusForbidden,
			ErrorTag: "token_banned",
			Message:  "Current token cannot be used",
		}
	case errors.Is(err, services.ErrBadPayload):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusBadRequest,
			ErrorTag: "bad_payload",
			Message:  "Payload could not be parsed",
		}
	default:
		return err
	}
}

func processPostServiceErrors(err error) error {
	switch {
	case errors.Is(err, services.ErrorPostDoesNotExist):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusNotFound,
			ErrorTag: "post_does_not_exist",
			Message:  "Post does not exist",
		}
	case errors.Is(err, services.ErrorPostWasDeleted):
		return &middleware.HttpErrorMessage{
			Code:     http.StatusConflict,
			ErrorTag: "post_was_deleted",
			Message:  "Post cannot be deleted",
		}
	default:
		return err
	}
}
