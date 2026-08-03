package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error)
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
		return nil, &errs.NotFoundError{ID: id.String(), Resource: "FindUserById"}
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) FindUserByLogin(ctx context.Context, login string) (*database.User, error) {
	user, err := repo.queries.FindUserByLogin(ctx, login)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &errs.NotFoundError{ID: login, Resource: "FindUserByLogin"}
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	user, err := repo.queries.FindUserByLoginOrEmail(
		ctx,
		database.FindUserByLoginOrEmailParams{
			Login: login,
			Email: email,
		})

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &errs.NotFoundError{ID: fmt.Sprintf("%s and %s", login, email), Resource: "FindUserById"}
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return &user, nil
}

func (repo *userRepository) AddUser(ctx context.Context, user *database.User) error {
	err := repo.queries.AddUser(
		ctx,
		database.AddUserParams{
			ID:           user.ID,
			Login:        user.Login,
			Email:        user.Email,
			PasswordHash: user.PasswordHash,
		})

	if IsUniqueViolation(err) {
		return fmt.Errorf("user with such credentials already exists: %w", errs.ErrAlreadyExists)
	}

	return err
}

func (repo *userRepository) GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	ids, err := repo.queries.GetSubscribers(ctx, userId)

	if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	return ids, nil
}

func (repo *userRepository) SubscribeUserTo(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	err := repo.queries.SubscribeUserTo(
		ctx,
		database.SubscribeUserToParams{
			SubID:  subId,
			AuthID: authId,
		})

	if IsUniqueViolation(err) {
		return fmt.Errorf("%s -> %s sub pair already exists: %w",
			subId,
			authId,
			errs.ErrAlreadyExists)
	}

	return err
}

func (repo *userRepository) UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	cnt, err := repo.queries.UnsubscribeUserFrom(
		ctx,
		database.UnsubscribeUserFromParams{
			SubID:  subId,
			AuthID: authId,
		})

	if cnt == 0 {
		return &errs.NotFoundError{
			ID:       fmt.Sprintf("%s -> %s", subId, authId),
			Resource: "UnsubscribeUserFrom",
		}
	}

	return err
}
