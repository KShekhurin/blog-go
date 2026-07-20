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
	CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error)
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

func (service *userService) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
	_, err := service.GetUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)

	if err == nil {
		return nil, ErrorUserExist
	}
	if !errors.Is(err, repositories.ErrorUserDoesNotExist) {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	hash, err := argon2id.CreateHash(userInfo.Password, service.params)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &database.User{
		ID:           uuid.New(),
		Login:        userInfo.Login,
		Email:        userInfo.Email,
		PasswordHash: hash,
	}
	err = service.userRepo.AddUser(ctx, newUser)

	if err != nil {
		if repositories.IsUniqueViolation(err) {
			return nil, ErrorUserExist
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return newUser, nil
}
