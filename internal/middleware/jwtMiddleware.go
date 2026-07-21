package middleware

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"strings"

	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const UserIDKey = "user_id"

type authHeader struct {
	HeaderValue string `header:"Authorization" binding:"required"`
}

func (h authHeader) tokenStr() (string, error) {
	data := strings.SplitN(h.HeaderValue, " ", 2)

	if len(data) != 2 {
		return "", errors.New("invalid auth tokenStr")
	}
	if !strings.EqualFold(data[0], "Bearer") {
		return "", errors.New("invalid auth scheme")
	}

	return strings.TrimSpace(data[1]), nil
}

type JwtMiddleware interface {
	Pass(c *gin.Context)
}

type jwtMiddleware struct {
	publicKey ed25519.PublicKey
}

func NewJwtMiddleware(publicKey ed25519.PublicKey) JwtMiddleware {
	return &jwtMiddleware{
		publicKey: publicKey,
	}
}

func (m *jwtMiddleware) Pass(c *gin.Context) {
	var header authHeader

	if err := c.ShouldBindHeader(&header); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tokenStr, err := header.tokenStr()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var claims services.TokenClaims

	token, err := jwt.ParseWithClaims(
		tokenStr,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			return m.publicKey, nil
		},
		jwt.WithIssuer(services.IssuerName),
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt())

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "expired"})
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if !token.Valid || claims.Type != services.AuthType {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(claims.Subject)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Set(UserIDKey, id)

	c.Next()
}
