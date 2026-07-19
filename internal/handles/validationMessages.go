package handles

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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

func BuildBindErrorMessage(err error) any {
	var syntaxErr *json.SyntaxError
	var unmarshalTypeErr *json.UnmarshalTypeError
	var validationErrs validator.ValidationErrors

	switch {
	case errors.As(err, &syntaxErr):
		return GeneralErrorMessage{
			Error:   "malformed_json",
			Message: fmt.Sprintf("offset: %d", syntaxErr.Offset),
		}
	case errors.As(err, &unmarshalTypeErr):
		return ValidationErrorMessage{
			Error: "wrong_type",
			Details: []DetailBody{
				{
					Field:   unmarshalTypeErr.Field,
					Code:    "wrong_type",
					Message: fmt.Sprintf("expected type %s", unmarshalTypeErr.Type),
				},
			},
		}
	case errors.As(err, &validationErrs):
		errDetails := make([]DetailBody, 0, len(validationErrs))

		for _, validationErr := range validationErrs {
			errDetails = append(errDetails, DetailBody{
				Field: strings.ToLower(validationErr.Field()),
				Code:  validationErr.ActualTag(),
			})
		}

		return ValidationErrorMessage{
			Error:   "validation_failed",
			Details: errDetails,
		}
	case errors.Is(err, io.EOF):
		return GeneralErrorMessage{
			Error:   "malformed_json",
			Message: "empty body",
		}
	case errors.Is(err, io.ErrUnexpectedEOF):
		return GeneralErrorMessage{
			Error:   "malformed_json",
			Message: "unexpected end of json input",
		}
	default:
		return GeneralErrorMessage{
			Error: "bad request",
		}
	}
}
