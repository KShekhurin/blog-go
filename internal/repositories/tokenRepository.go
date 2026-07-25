package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type TokenRepository interface {
	AddToken(ctx context.Context, jti uuid.UUID, userId uuid.UUID, expiresAt time.Time) error
	TryToDeleteToken(ctx context.Context, jti uuid.UUID) (isSuccess bool, err error)
	DeleteAllUserTokens(ctx context.Context, userId uuid.UUID) error
}

type tokenRepository struct {
	query *database.Queries
}

func NewTokenRepository(query *database.Queries) TokenRepository {
	return &tokenRepository{query: query}
}

func (r tokenRepository) AddToken(ctx context.Context, jti uuid.UUID, userId uuid.UUID, expiresAt time.Time) error {
	err := r.query.AddRefreshToken(ctx, database.AddRefreshTokenParams{
		Jti:    jti,
		UserID: userId,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
	})

	if err != nil {
		if IsUniqueViolation(err) {
			return fmt.Errorf("such refresh token already exists: %w", ErrorUniqueViolation)
		}
	}

	return err
}

func (r tokenRepository) TryToDeleteToken(ctx context.Context, jti uuid.UUID) (isSuccess bool, err error) {
	cnt, err := r.query.DeleteRefreshToken(ctx, jti)

	if err != nil {
		return false, err
	}

	return cnt != 0, nil
}

func (r tokenRepository) DeleteAllUserTokens(ctx context.Context, userId uuid.UUID) error {
	err := r.query.DeleteAllRefreshTokensByUserId(ctx, userId)
	return err
}
