package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		case errors.Is(err, services.ErrorInvalidCredentials):
			c.JSON(http.StatusUnauthorized,
				GeneralErrorMessage{
					Error:   "invalid_credentials",
					Message: "Invalid Credentials",
				})
		case errors.Is(err, services.ErrorUserExist):
			c.JSON(http.StatusConflict,
				GeneralErrorMessage{
					Error:   "user_exists",
					Message: "User already exists",
				})
		default:
			c.JSON(http.StatusInternalServerError,
				GeneralErrorMessage{
					Error:   "internal_server_error",
					Message: "Internal Server Error",
				})
		}

		c.Abort()
	}
}
