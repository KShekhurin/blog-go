package services

import (
	"context"
	"errors"
	"testing"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
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
		service := NewUserService(stub, argon2id.DefaultParams)

		user, err := service.CreateUser(ctx, userInfo)

		assert.ErrorIs(t, err, ErrorUserExist)
		assert.Nil(t, user)
	})

	t.Run("returns wrapped error when existence check fails unexpectedly", func(t *testing.T) {
		stub := &userRepositoryStub{
			findErr: errors.New("connection refused"),
		}
		service := NewUserService(stub, argon2id.DefaultParams)

		user, err := service.CreateUser(ctx, userInfo)

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to check user existence")
		assert.Nil(t, user)
	})

	t.Run("returns ErrorUserExist on unique violation", func(t *testing.T) {
		stub := &userRepositoryStub{
			findErr: repositories.ErrorDoesNotExist,
			addErr:  &pgconn.PgError{Code: "23505"},
		}
		service := NewUserService(stub, argon2id.DefaultParams)

		user, err := service.CreateUser(ctx, userInfo)

		assert.ErrorIs(t, err, ErrorUserExist)
		assert.Nil(t, user)
	})

	t.Run("returns created user on success", func(t *testing.T) {
		stub := &userRepositoryStub{
			findErr: repositories.ErrorDoesNotExist,
			addErr:  nil,
		}
		service := NewUserService(stub, argon2id.DefaultParams)

		user, err := service.CreateUser(ctx, userInfo)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.Equal(t, userInfo.Login, user.Login)
		assert.Equal(t, userInfo.Email, user.Email)

		match, err := argon2id.ComparePasswordAndHash(userInfo.Password, user.PasswordHash)
		require.NoError(t, err)
		assert.True(t, match)
	})
}
