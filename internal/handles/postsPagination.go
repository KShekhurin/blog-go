package handles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var (
	errWrongCursorFormat = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "invalid_cursor",
		Message:  "Invalid cursor format",
	}
	errLimitMustBeNumber = middleware.HttpErrorMessage{
		Code:     http.StatusBadRequest,
		ErrorTag: "invalid_limit",
		Message:  "Limit must be a positive number",
	}
)

func decodeCursor(cursorEncodedStr string) (*webModels.PostPaginationCursor, error) {
	if cursorEncodedStr == "" {
		return nil, nil
	}

	cursorDecodedStr, err := base64.StdEncoding.DecodeString(cursorEncodedStr)
	if err != nil {
		return nil, &errWrongCursorFormat
	}

	var cursor webModels.PostPaginationCursor
	if err := json.Unmarshal(cursorDecodedStr, &cursor); err != nil {
		return nil, &errWrongCursorFormat
	}

	validate := binding.Validator.Engine().(*validator.Validate)

	if err := validate.Struct(&cursor); err != nil {
		return nil, &errWrongCursorFormat
	}

	return &cursor, nil
}

func encodeCursor(cursor *webModels.PostPaginationCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}

	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("could not marshal cursor %s: %w", cursor, err)
	}

	cursorEncodedStr := base64.StdEncoding.EncodeToString(data)

	return cursorEncodedStr, nil
}

func getCursorAndLimit(ctx *gin.Context) (*webModels.PostPaginationCursor, int, error) {
	cursorEncodedStr := ctx.DefaultQuery("cursor", "")
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if err != nil {
		return nil, 0, &errLimitMustBeNumber
	}

	var cursor *webModels.PostPaginationCursor = nil

	if cursorEncodedStr != "" {
		cursor, err = decodeCursor(cursorEncodedStr)
		if err != nil {
			return nil, 0, err
		}
	}

	return cursor, limit, nil
}
