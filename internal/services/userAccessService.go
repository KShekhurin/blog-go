package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
)

type UserService interface {
	GetUserByLogin(ctx context.Context, login string)
	GetUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error)
	GetUserByUUID(ctx context.Context)
	CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (uuid.UUID, error)
}

var (
	ErrorUserExist       = errors.New("user already exists")
	ErrorBadPayload      = errors.New("bad payload")
	ErrorInvalidPassword = errors.New("invalid password")
)

type userService struct {
	userRepo repositories.UserRepository
	params   *argon2id.Params
}

func NewUserService(userRepo repositories.UserRepository, hashParams *argon2id.Params) UserService {
	return &userService{
		userRepo: userRepo,
		params:   hashParams,
	}
}

func (service *userService) GetUserByLogin(ctx context.Context, login string) {

}

func (service *userService) GetUserByUUID(ctx context.Context) {

}

func (service *userService) GetUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	usr, err := service.userRepo.FindUserByLoginOrEmail(ctx, login, email)

	return usr, err
}

func (service *userService) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (uuid.UUID, error) {
	_, err := service.GetUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)

	if err == nil {
		return uuid.UUID{}, ErrorUserExist
	}
	if !errors.Is(err, repositories.ErrorUserDoesNotExist) {
		return uuid.UUID{}, fmt.Errorf("failed to check user existence: %w", err)
	}

	hash, err := argon2id.CreateHash(userInfo.Password, service.params)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to hash password: %w", err)
	}

	newUuid := uuid.New()
	err = service.userRepo.AddUser(ctx, &database.User{
		ID:           newUuid,
		Login:        userInfo.Login,
		Email:        userInfo.Email,
		PasswordHash: hash,
	})

	if err != nil {
		if repositories.IsUniqueViolation(err) {
			return uuid.UUID{}, ErrorUserExist
		}
		return uuid.UUID{}, fmt.Errorf("failed to create user: %w", err)
	}

	return newUuid, nil
}
