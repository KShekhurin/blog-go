//go:build unit

package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupErrorRouter(err error) *gin.Engine {
	r := gin.New()
	r.Use(ErrorMiddleware())
	r.GET("/test", func(c *gin.Context) {
		if err != nil {
			c.Error(err)
		}
	})
	return r
}

func performRequest(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestErrorMiddleware_NoErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupErrorRouter(nil)
	w := performRequest(r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestErrorMiddleware_SyntaxError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	syntaxErr := &json.SyntaxError{}

	r := setupErrorRouter(syntaxErr)
	w := performRequest(r)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp GeneralErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "malformed_json", resp.Error)
	assert.Equal(t, "Request body contains invalid JSON.", resp.Message)
}

func TestErrorMiddleware_UnmarshalTypeError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unmarshalTypeErr := &json.UnmarshalTypeError{
		Field: "age",
	}

	r := setupErrorRouter(unmarshalTypeErr)
	w := performRequest(r)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp ValidationErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "wrong_type", resp.Error)
	require.Len(t, resp.Details, 1)
	assert.Equal(t, "age", resp.Details[0].Field)
	assert.Equal(t, "wrong_type", resp.Details[0].Code)
	assert.Contains(t, resp.Details[0].Message, "expected type")
}

func TestErrorMiddleware_ValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validate := validator.New()

	type TestStruct struct {
		Email string `validate:"required,email"`
		Age   int    `validate:"required"`
	}

	var ts TestStruct
	err := validate.Struct(ts)
	require.Error(t, err)

	validationErrs, ok := err.(validator.ValidationErrors)
	require.True(t, ok)

	r := setupErrorRouter(validationErrs)
	w := performRequest(r)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp ValidationErrorMessage
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "validation_failed", resp.Error)
	require.Len(t, resp.Details, 2)

	assert.Equal(t, "email", resp.Details[0].Field)
	assert.Equal(t, "Email is required", resp.Details[0].Message)

	assert.Equal(t, "age", resp.Details[1].Field)
	assert.Equal(t, "Age is required", resp.Details[1].Message)
}

func TestErrorMiddleware_EOF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupErrorRouter(io.EOF)
	w := performRequest(r)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp GeneralErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "empty_body", resp.Error)
	assert.Equal(t, "The request body must contain required fields", resp.Message)
}

func TestErrorMiddleware_UnexpectedEOF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupErrorRouter(io.ErrUnexpectedEOF)
	w := performRequest(r)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp GeneralErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "malformed_json", resp.Error)
	assert.Equal(t, "unexpected end of json input", resp.Message)
}

func TestErrorMiddleware_InternalServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupErrorRouter(fmt.Errorf("some unknown error"))
	w := performRequest(r)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var resp GeneralErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "internal_server_error", resp.Error)
	assert.Equal(t, "Internal Server Error", resp.Message)
}

func TestErrorMiddleware_HttpError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupErrorRouter(&HttpErrorMessage{
		Message:  "some error message",
		ErrorTag: "error",
		Code:     http.StatusTeapot,
	})
	w := performRequest(r)

	require.Equal(t, http.StatusTeapot, w.Code)
	var resp GeneralErrorMessage
	err := json.Unmarshal(w.Body.Bytes(), &resp)

	require.NoError(t, err)
	assert.Equal(t, "some error message", resp.Message)
	assert.Equal(t, "error", resp.Error)
}

// mockFieldError implements validator.FieldError for testing messageForTag
type mockFieldError struct {
	tag   string
	field string
}

var _ validator.FieldError = mockFieldError{}

func (m mockFieldError) Tag() string                    { return m.tag }
func (m mockFieldError) ActualTag() string              { return m.tag }
func (m mockFieldError) Namespace() string              { return "" }
func (m mockFieldError) StructNamespace() string        { return "" }
func (m mockFieldError) Field() string                  { return m.field }
func (m mockFieldError) StructField() string            { return m.field }
func (m mockFieldError) Value() interface{}             { return nil }
func (m mockFieldError) Param() string                  { return "" }
func (m mockFieldError) Kind() reflect.Kind             { return reflect.String }
func (m mockFieldError) Type() reflect.Type             { return nil }
func (m mockFieldError) Error() string                  { return "validation error" }
func (m mockFieldError) Translate(ut.Translator) string { return "" }

func TestMessageForTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		field    string
		expected string
	}{
		{
			name:     "required",
			tag:      "required",
			field:    "Email",
			expected: "Email is required",
		},
		{
			name:     "email",
			tag:      "email",
			field:    "Email",
			expected: "Email must be a proper email address",
		},
		{
			name:     "required_if_no_email",
			tag:      "required_if_no_email",
			field:    "Login",
			expected: "Login is required if no email was presented",
		},
		{
			name:     "required_if_no_login",
			tag:      "required_if_no_login",
			field:    "Email",
			expected: "Email is required if no login was presented",
		},
		{
			name:     "unknown tag min",
			tag:      "min",
			field:    "Password",
			expected: "Password failed validation on min",
		},
		{
			name:     "unknown tag gte",
			tag:      "gte",
			field:    "Age",
			expected: "Age failed validation on gte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldErr := mockFieldError{tag: tt.tag, field: tt.field}
			assert.Equal(t, tt.expected, messageForTag(fieldErr))
		})
	}
}
