package services

import (
	"context"
	"testing"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userRepositoryStub struct {
	findUser *database.User
	findErr  error
	addErr   error
}

func (s *userRepositoryStub) FindUserById(ctx context.Context, id uuid.UUID) (*database.User, error) {
	return nil, nil
}

func (s *userRepositoryStub) FindUserByLogin(ctx context.Context, login string) (*database.User, error) {
	return nil, nil
}

func (s *userRepositoryStub) FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	return s.findUser, s.findErr
}

func (s *userRepositoryStub) AddUser(ctx context.Context, user *database.User) error {
	return s.addErr
}

func (s *userRepositoryStub) SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	return nil
}

func (s *userRepositoryStub) UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	return nil
}

func TestCreateUser(t *testing.T) {
	ctx := context.Background()
	userInfo := &webModels.UserRegisterInfo{
		Login:    "tom",
		Email:    "tom@gmail.com",
		Password: "password",
	}

	t.Run("returns ErrorUserExist when user already found", func(t *testing.T) {
		stub := &userRepositoryStub{
			findUser: &database.User{},
			findErr:  nil,
		}
		service := NewUserService(stub)

		id, err := service.CreateUser(ctx, userInfo)

		assert.ErrorIs(t, err, ErrorUserExist)
		assert.Equal(t, uuid.UUID{}, id)
	})

	t.Run("returns ErrorUserExist on unique violation", func(t *testing.T) {
		stub := &userRepositoryStub{
			findErr: pgx.ErrNoRows,
			addErr:  &pgconn.PgError{Code: "23505"},
		}
		service := NewUserService(stub)

		id, err := service.CreateUser(ctx, userInfo)

		assert.ErrorIs(t, err, ErrorUserExist)
		assert.Equal(t, uuid.UUID{}, id)
	})

	t.Run("returns generated UUID on success", func(t *testing.T) {
		stub := &userRepositoryStub{
			findErr: pgx.ErrNoRows,
			addErr:  nil,
		}
		service := NewUserService(stub)

		id, err := service.CreateUser(ctx, userInfo)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.UUID{}, id)
	})
}
