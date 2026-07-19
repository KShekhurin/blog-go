package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrorUserDoesNotExist = errors.New("user does not exists")
	ErrorUniqueViolation  = errors.New("unique violation")
)

func IsUniqueViolation(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

type UserRepository interface {
	FindUserById(ctx context.Context, id uuid.UUID) (*database.User, error)
	FindUserByLogin(ctx context.Context, login string) (*database.User, error)
	FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error)
	AddUser(ctx context.Context, user *database.User) error
	SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
	UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
}

type userRepository struct {
	queries *database.Queries
}

func NewUserRepository(queries *database.Queries) UserRepository {
	return &userRepository{
		queries: queries,
	}
}

func (repo *userRepository) FindUserById(ctx context.Context, id uuid.UUID) (*database.User, error) {
	user, err := repo.queries.FindUserById(ctx, id)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrorUserDoesNotExist
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) FindUserByLogin(ctx context.Context, login string) (*database.User, error) {
	user, err := repo.queries.FindUserByLogin(ctx, login)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrorUserDoesNotExist
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	user, err := repo.queries.FindUserByLoginOrEmail(
		ctx,
		database.FindUserByLoginOrEmailParams{
			login,
			email,
		})

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrorUserDoesNotExist
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) AddUser(ctx context.Context, user *database.User) error {
	err := repo.queries.CreateUser(
		ctx,
		database.CreateUserParams{
			ID:           user.ID,
			Login:        user.Login,
			Email:        user.Email,
			PasswordHash: user.PasswordHash,
		})

	if IsUniqueViolation(err) {
		return ErrorUniqueViolation
	}

	return err
}

func (repo *userRepository) SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	err := repo.queries.SubscribeUserTo(
		ctx,
		database.SubscribeUserToParams{
			sub_id,
			auth_id,
		})

	if IsUniqueViolation(err) {
		return ErrorUniqueViolation
	}

	return err
}

func (repo *userRepository) UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	err := repo.queries.UnsubscribeUserFrom(
		ctx,
		database.UnsubscribeUserFromParams{
			sub_id,
			auth_id,
		})

	if IsUniqueViolation(err) {
		return ErrorUniqueViolation
	}

	return err
}
