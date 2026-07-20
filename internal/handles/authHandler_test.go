package handles

import (
	"context"
	"errors"
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

// --- stubs ---

type authServiceStub struct {
	authenticateUserFn func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error)
	signJWTFn          func(ctx context.Context, user *database.User) (*services.TokenPair, error)

	authenticateCalls int
	signJWTCalls      int
}

var _ services.AuthService = (*authServiceStub)(nil)

func (s *authServiceStub) AuthenticateUser(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
	s.authenticateCalls++
	return s.authenticateUserFn(ctx, userInfo)
}

func (s *authServiceStub) SignJWT(ctx context.Context, user *database.User) (*services.TokenPair, error) {
	s.signJWTCalls++
	return s.signJWTFn(ctx, user)
}

type userServiceStub struct {
	createUserFn func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error)

	createUserCalls int
}

var _ services.UserService = (*userServiceStub)(nil)

func (s *userServiceStub) GetUserByLogin(ctx context.Context, login string) {}

func (s *userServiceStub) GetUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	return nil, nil
}

func (s *userServiceStub) GetUserByUUID(ctx context.Context) {}

func (s *userServiceStub) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
	s.createUserCalls++
	if s.createUserFn == nil {
		return nil, nil
	}
	return s.createUserFn(ctx, userInfo)
}

// --- tests ---

var (
	testUser       = &database.User{ID: uuid.New(), Login: "tom", Email: "tom@gmail.com", PasswordHash: "hash"}
	validLoginBody = `{"login":"tom","password":"password"}`
)

func TestAuthHandler_Login_ErrorsPutIntoContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	errAuthenticate := errors.New("authenticate failed")
	errSign := errors.New("sign failed")

	tests := []struct {
		name             string
		body             string
		authenticateUser func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error)
		signJWT          func(ctx context.Context, user *database.User) (*services.TokenPair, error)
		wantErr          error // nil means "any error", checked only by count
		wantAuthCalls    int
		wantSignCalls    int
	}{
		{
			name: "ShouldBindBodyWithJSON error",
			body: `{"login":"tom",`, // malformed JSON
			authenticateUser: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
				return nil, errors.New("AuthenticateUser must not be called")
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errors.New("SignJWT must not be called")
			},
			wantErr:       nil,
			wantAuthCalls: 0,
			wantSignCalls: 0,
		},
		{
			name: "AuthenticateUser error",
			body: validLoginBody,
			authenticateUser: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
				return nil, errAuthenticate
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errors.New("SignJWT must not be called")
			},
			wantErr:       errAuthenticate,
			wantAuthCalls: 1,
			wantSignCalls: 0,
		},
		{
			name: "SignJWT error",
			body: validLoginBody,
			authenticateUser: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
				return testUser, nil
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errSign
			},
			wantErr:       errSign,
			wantAuthCalls: 1,
			wantSignCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authStub := &authServiceStub{
				authenticateUserFn: tt.authenticateUser,
				signJWTFn:          tt.signJWT,
			}
			handler := NewAuthHandler(&userServiceStub{}, authStub)

			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")

			handler.Login(ctx)

			require.Len(t, ctx.Errors, 1, "handler must put exactly one error into the context")
			if tt.wantErr != nil {
				assert.ErrorIs(t, ctx.Errors[0].Err, tt.wantErr)
			}
			assert.Equal(t, tt.wantAuthCalls, authStub.authenticateCalls)
			assert.Equal(t, tt.wantSignCalls, authStub.signJWTCalls)
			assert.Equal(t, 0, w.Body.Len(), "handler must not write a response body on error")
		})
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokenPair := &services.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}

	authStub := &authServiceStub{
		authenticateUserFn: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
			assert.Equal(t, "tom", userInfo.Login)
			assert.Equal(t, "password", userInfo.Password)
			return testUser, nil
		},
		signJWTFn: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
			assert.Same(t, testUser, user, "SignJWT must receive the user returned by AuthenticateUser")
			return tokenPair, nil
		},
	}
	handler := NewAuthHandler(&userServiceStub{}, authStub)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(validLoginBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Login(ctx)

	assert.Empty(t, ctx.Errors, "no errors expected on success")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"access_token":"access-token","refresh_token":"refresh-token"}`, w.Body.String())
	assert.Equal(t, 1, authStub.authenticateCalls)
	assert.Equal(t, 1, authStub.signJWTCalls)
}

var validRegisterBody = `{"login":"tom","email":"tom@gmail.com","password":"password"}`

func TestAuthHandler_Register_ErrorsPutIntoContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	errCreateUser := errors.New("create user failed")
	errSign := errors.New("sign failed")

	tests := []struct {
		name             string
		body             string
		createUser       func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error)
		signJWT          func(ctx context.Context, user *database.User) (*services.TokenPair, error)
		wantErr          error // nil means "any error", checked only by count
		wantCreateCalls  int
		wantSignJWTCalls int
	}{
		{
			name: "ShouldBindBodyWithJSON error",
			body: `{"login":"tom",`, // malformed JSON
			createUser: func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
				return nil, errors.New("CreateUser must not be called")
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errors.New("SignJWT must not be called")
			},
			wantErr:          nil,
			wantCreateCalls:  0,
			wantSignJWTCalls: 0,
		},
		{
			name: "ShouldBindBodyWithJSON validation error",
			body: `{"login":"tom","password":"password"}`, // missing required email
			createUser: func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
				return nil, errors.New("CreateUser must not be called")
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errors.New("SignJWT must not be called")
			},
			wantErr:          nil,
			wantCreateCalls:  0,
			wantSignJWTCalls: 0,
		},
		{
			name: "CreateUser error",
			body: validRegisterBody,
			createUser: func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
				return nil, errCreateUser
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errors.New("SignJWT must not be called")
			},
			wantErr:          errCreateUser,
			wantCreateCalls:  1,
			wantSignJWTCalls: 0,
		},
		{
			name: "SignJWT error",
			body: validRegisterBody,
			createUser: func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
				return testUser, nil
			},
			signJWT: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
				return nil, errSign
			},
			wantErr:          errSign,
			wantCreateCalls:  1,
			wantSignJWTCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userStub := &userServiceStub{createUserFn: tt.createUser}
			authStub := &authServiceStub{
				authenticateUserFn: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
					return nil, errors.New("AuthenticateUser must not be called")
				},
				signJWTFn: tt.signJWT,
			}
			handler := NewAuthHandler(userStub, authStub)

			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")

			handler.Register(ctx)

			require.Len(t, ctx.Errors, 1, "handler must put exactly one error into the context")
			if tt.wantErr != nil {
				assert.ErrorIs(t, ctx.Errors[0].Err, tt.wantErr)
			}
			assert.Equal(t, tt.wantCreateCalls, userStub.createUserCalls)
			assert.Equal(t, tt.wantSignJWTCalls, authStub.signJWTCalls)
			assert.Equal(t, 0, w.Body.Len(), "handler must not write a response body on error")
		})
	}
}

func TestAuthHandler_Register_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokenPair := &services.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}

	userStub := &userServiceStub{
		createUserFn: func(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
			assert.Equal(t, "tom", userInfo.Login)
			assert.Equal(t, "tom@gmail.com", userInfo.Email)
			assert.Equal(t, "password", userInfo.Password)
			return testUser, nil
		},
	}
	authStub := &authServiceStub{
		authenticateUserFn: func(ctx context.Context, userInfo *webModels.UserLoginInfo) (*database.User, error) {
			return nil, errors.New("AuthenticateUser must not be called")
		},
		signJWTFn: func(ctx context.Context, user *database.User) (*services.TokenPair, error) {
			assert.Same(t, testUser, user, "SignJWT must receive the user returned by CreateUser")
			return tokenPair, nil
		},
	}
	handler := NewAuthHandler(userStub, authStub)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(validRegisterBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Register(ctx)

	assert.Empty(t, ctx.Errors, "no errors expected on success")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.JSONEq(t, `{"access_token":"access-token","refresh_token":"refresh-token"}`, w.Body.String())
	assert.Equal(t, 1, userStub.createUserCalls)
	assert.Equal(t, 1, authStub.signJWTCalls)
}
