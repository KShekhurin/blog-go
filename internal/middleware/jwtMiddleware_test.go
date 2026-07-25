package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

type testKeys struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func generateTestKeys(t *testing.T) testKeys {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return testKeys{public: pub, private: priv}
}

// signedToken builds and signs a token; pass nil claims to get valid defaults
func signedToken(t *testing.T, key ed25519.PrivateKey, method jwt.SigningMethod, claims *services.TokenClaims) string {
	t.Helper()
	if claims == nil {
		claims = &services.TokenClaims{
			Type: services.AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    services.IssuerName,
				Subject:   uuid.NewString(),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
		}
	}
	tokenStr, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return tokenStr
}

// setupRouter wires the middleware and a probe handler that echoes the stored user id
func setupJwtRouter(publicKey ed25519.PublicKey) *gin.Engine {
	r := gin.New()
	mw := NewJwtMiddleware(publicKey)

	r.GET("/protected", mw.Pass, func(c *gin.Context) {
		id, exists := c.Get(UserIDKey)
		if !exists {
			c.Status(http.StatusOK)
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": id.(uuid.UUID).String()})
	})
	return r
}

func performJwtRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestJwtMiddlewarePass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keys := generateTestKeys(t)
	userID := uuid.New()

	t.Run("missing authorization header", func(t *testing.T) {
		r := setupJwtRouter(keys.public)
		w := performJwtRequest(r, "")

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
	})

	t.Run("malformed header content", func(t *testing.T) {
		tests := []struct {
			name   string
			header string
		}{
			{"no scheme", "just-a-token"},
			{"wrong scheme", "Basic dGVzdA=="},
			{"empty bearer token", "Bearer "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := setupJwtRouter(keys.public)
				w := performJwtRequest(r, tt.header)

				assert.Equal(t, http.StatusUnauthorized, w.Code)
				assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
			})
		}
	})

	t.Run("invalid token or bad signature", func(t *testing.T) {
		tests := []struct {
			name  string
			token string
		}{
			{"garbage string", "not.a.token"},
			{
				"bad signature",
				// valid token signed by a *different* key
				signedToken(t, generateTestKeys(t).private, jwt.SigningMethodEdDSA, nil),
			},
			{
				"subject is not a uuid",
				signedToken(t, keys.private, jwt.SigningMethodEdDSA, &services.TokenClaims{
					Type: services.AuthType,
					RegisteredClaims: jwt.RegisteredClaims{
						Issuer:    services.IssuerName,
						Subject:   "not-a-uuid",
						IssuedAt:  jwt.NewNumericDate(time.Now()),
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
					},
				}),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := setupJwtRouter(keys.public)
				w := performJwtRequest(r, "Bearer "+tt.token)

				assert.Equal(t, http.StatusUnauthorized, w.Code)
				assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
			})
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := signedToken(t, keys.private, jwt.SigningMethodEdDSA, &services.TokenClaims{
			Type: services.AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    services.IssuerName,
				Subject:   userID.String(),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			},
		})

		r := setupJwtRouter(keys.public)
		w := performJwtRequest(r, "Bearer "+token)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(t, `{"error": "expired"}`, w.Body.String())
	})

	t.Run("claim type is not auth", func(t *testing.T) {
		token := signedToken(t, keys.private, jwt.SigningMethodEdDSA, &services.TokenClaims{
			Type: services.RefreshType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    services.IssuerName,
				Subject:   userID.String(),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
		})

		r := setupJwtRouter(keys.public)
		w := performJwtRequest(r, "Bearer "+token)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
	})

	t.Run("none algorithm in header", func(t *testing.T) {
		token, err := jwt.NewWithClaims(jwt.SigningMethodNone, &services.TokenClaims{
			Type: services.AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    services.IssuerName,
				Subject:   userID.String(),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
		}).SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		r := setupJwtRouter(keys.public)
		w := performJwtRequest(r, "Bearer "+token)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
	})

	t.Run("valid token sets user id", func(t *testing.T) {
		token := signedToken(t, keys.private, jwt.SigningMethodEdDSA, &services.TokenClaims{
			Type: services.AuthType,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    services.IssuerName,
				Subject:   userID.String(),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
		})

		r := setupJwtRouter(keys.public)
		w := performJwtRequest(r, "Bearer "+token)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"user_id": "`+userID.String()+`"}`, w.Body.String())
	})
}
