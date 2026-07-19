package handles

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userServiceStub struct {
	id  uuid.UUID
	err error
}

func (u *userServiceStub) GetUserByLogin(ctx context.Context, login string) {}

func (u *userServiceStub) GetUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {

}

func (u *userServiceStub) GetUserByUUID(ctx context.Context) {}

func (u *userServiceStub) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (uuid.UUID, error) {
	return u.id, u.err
}

func TestCreateUserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	successStub := &userServiceStub{
		id:  uuid.New(),
		err: nil,
	}

	userExisitsStub := &userServiceStub{
		id:  uuid.New(),
		err: services.ErrorUserExist,
	}

	internalErrorStub := &userServiceStub{
		id:  uuid.New(),
		err: fmt.Errorf("some error"),
	}

	tests := []struct {
		name      string
		userInput string
		wantCode  int
		wantBody  string
		stub      *userServiceStub
	}{
		{
			"returns ID",
			`{"login": "tom", "email": "tom@gmail.com", "password": "password"}`,
			http.StatusCreated,
			fmt.Sprintf(`{"id": "%s"}`, successStub.id),
			successStub,
		},
		{
			"no login field",
			`{"email": "tom@gmail.com", "password": "password"}`,
			http.StatusBadRequest,
			`{
					"error": "validation_failed", 
					"details": [
						{"field": "login", "code": "required"}
					]}`,
			nil,
		},
		{
			"user exists",
			`{"login": "tom", "email": "tom@gmail.com", "password": "password"}`,
			http.StatusConflict,
			`{"error": "user already exists", "message": "user with such login or email already exists"}`,
			userExisitsStub,
		},
		{
			"internal error",
			`{"login": "tom", "email": "tom@gmail.com", "password": "password"}`,
			http.StatusInternalServerError,
			`{"error": "internal server error"}`,
			internalErrorStub,
		},
		{
			"empty body",
			``,
			http.StatusBadRequest,
			`{"error": "malformed_json", "message": "empty body"}`,
			nil,
		},
		{
			"unexpected end of json input",
			`{"login": "tom"`,
			http.StatusBadRequest,
			`{"error": "malformed_json", "message": "unexpected end of json input"}`,
			nil,
		},
		{
			"malformed json",
			`{"login": "tom",}`,
			http.StatusBadRequest,
			`{"error": "malformed_json", "message": "offset: 17"}`,
			nil,
		},
		{
			"wrong type",
			`{"login": 123, "email": "tom@gmail.com", "password": "password"}`,
			http.StatusBadRequest,
			`{
				"error": "wrong_type",
				"details": [
					{"field": "login", "code": "wrong_type", "message": "expected type string"}
				]}`,
			nil,
		},
		{
			"invalid email",
			`{"login": "tom", "email": "not-an-email", "password": "password"}`,
			http.StatusBadRequest,
			`{
				"error": "validation_failed",
				"details": [
					{"field": "email", "code": "email"}
				]}`,
			nil,
		},
		{
			"missing password",
			`{"login": "tom", "email": "tom@gmail.com"}`,
			http.StatusBadRequest,
			`{
				"error": "validation_failed",
				"details": [
					{"field": "password", "code": "required"}
				]}`,
			nil,
		},
		{
			"multiple missing fields",
			`{"login": "tom"}`,
			http.StatusBadRequest,
			`{
				"error": "validation_failed",
				"details": [
					{"field": "email", "code": "required"},
					{"field": "password", "code": "required"}
				]}`,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.Default()

			handle := NewAuthHandler(tt.stub)

			r.POST("/api/v1/user", handle.Register)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/user", strings.NewReader(tt.userInput))
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantCode, w.Code)
			assert.JSONEq(t, tt.wantBody, w.Body.String())
		})
	}
}
