package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type DetailBody struct {
	Field   string `json:"field" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Message string `json:"message,omitempty"`
}

type ValidationErrorMessage struct {
	Error   string       `json:"error" binding:"required" default:"validation_failed"`
	Details []DetailBody `json:"details" binding:"required"`
}

type GeneralErrorMessage struct {
	Error   string `json:"error" binding:"required"`
	Message string `json:"message,omitempty"`
}

type HttpErrorMessage struct {
	ErrorTag string
	Message  string
	Code     int
}

func (h *HttpErrorMessage) Error() string {
	return fmt.Sprintf("Code: %d, Error: %s, Message: %s", h.Code, h.ErrorTag, h.Message)
}

func messageForTag(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "email":
		return fmt.Sprintf("%s must be a proper email address", err.Field())
	case "required_if_no_email":
		return fmt.Sprintf("%s is required if no email was presented", err.Field())
	case "required_if_no_login":
		return fmt.Sprintf("%s is required if no login was presented", err.Field())
	default:
		return fmt.Sprintf("%s failed validation on %s", err.Field(), err.Tag())
	}
}

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError
		var validationErrs validator.ValidationErrors
		var httpError *HttpErrorMessage

		switch {
		case errors.As(err, &syntaxErr):
			c.JSON(http.StatusBadRequest,
				GeneralErrorMessage{
					Error:   "malformed_json",
					Message: "Request body contains invalid JSON.",
				})
		case errors.As(err, &unmarshalTypeErr):
			c.JSON(http.StatusBadRequest,
				ValidationErrorMessage{
					Error: "wrong_type",
					Details: []DetailBody{
						{
							Field:   unmarshalTypeErr.Field,
							Code:    "wrong_type",
							Message: fmt.Sprintf("expected type %s", unmarshalTypeErr.Type),
						},
					},
				})
		case errors.As(err, &validationErrs):
			errDetails := make([]DetailBody, 0, len(validationErrs))

			for _, validationErr := range validationErrs {
				errDetails = append(errDetails, DetailBody{
					Field:   strings.ToLower(validationErr.Field()),
					Message: messageForTag(validationErr),
				})
			}

			c.JSON(http.StatusBadRequest,
				ValidationErrorMessage{
					Error:   "validation_failed",
					Details: errDetails,
				})
		case errors.Is(err, io.EOF):
			c.JSON(http.StatusBadRequest,
				GeneralErrorMessage{
					Error:   "empty_body",
					Message: "The request body must contain required fields",
				})
		case errors.Is(err, io.ErrUnexpectedEOF):
			c.JSON(http.StatusBadRequest,
				GeneralErrorMessage{
					Error:   "malformed_json",
					Message: "unexpected end of json input",
				})
		case errors.Is(err, services.ErrorUnauthorized):
			c.JSON(http.StatusUnauthorized,
				GeneralErrorMessage{
					Error:   "unauthorized",
					Message: "You are not authorized to perform this operation",
				})
		case errors.Is(err, context.DeadlineExceeded):
			c.JSON(http.StatusGatewayTimeout, GeneralErrorMessage{
				Error:   "request_timeout",
				Message: "The request took too long to process",
			})
		case errors.Is(err, context.Canceled):
			return
		case errors.As(err, &httpError):
			c.JSON(httpError.Code, GeneralErrorMessage{
				Error:   httpError.ErrorTag,
				Message: httpError.Message,
			})
		default:
			slog.ErrorContext(c.Request.Context(), "unexpected error", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError,
				GeneralErrorMessage{
					Error:   "internal_server_error",
					Message: "Internal Server Error",
				})
		}

		c.Abort()
	}
}
