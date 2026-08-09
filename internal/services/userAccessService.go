package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
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
	SubscribeTo(ctx context.Context, whoId uuid.UUID, toWhomId uuid.UUID) error
	UnsubscribeFrom(ctx context.Context, whoId uuid.UUID, fromWhomId uuid.UUID) error
}

var (
	ErrNotSubscribed     = errors.New("user is not subscribed")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrAlreadySubscribed = errors.New("user already subscribed")
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

func (s *userService) GetUserByLogin(ctx context.Context, login string) {

}

func (s *userService) GetUserByUUID(ctx context.Context) {

}

func (s *userService) GetUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	usr, err := s.userRepo.FindUserByLoginOrEmail(ctx, login, email)

	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return usr, nil
}

func (s *userService) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
	_, err := s.GetUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)

	if err == nil {
		return nil, fmt.Errorf("user with such credentials already exists: %w", ErrUserAlreadyExists)
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	hash, err := argon2id.CreateHash(userInfo.Password, s.params)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &database.User{
		ID:           uuid.New(),
		Login:        userInfo.Login,
		Email:        userInfo.Email,
		PasswordHash: hash,
	}
	err = s.userRepo.AddUser(ctx, newUser)

	if err != nil {
		if errors.Is(err, errs.ErrAlreadyExists) { //handles dirty write
			return nil, fmt.Errorf("user with such credentials already exists: %w", ErrUserAlreadyExists)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return newUser, nil
}

func (s *userService) SubscribeTo(ctx context.Context, whoId uuid.UUID, toWhomId uuid.UUID) error {
	err := s.userRepo.SubscribeUserTo(ctx, whoId, toWhomId)

	if err != nil {
		if errors.Is(err, errs.ErrAlreadyExists) {
			return fmt.Errorf("user already subscribed: %w", ErrAlreadySubscribed)
		}
		return fmt.Errorf("failed to subscribe to user: %w", err)
	}

	return nil
}

func (s *userService) UnsubscribeFrom(ctx context.Context, whoId uuid.UUID, fromWhomId uuid.UUID) error {
	err := s.userRepo.UnsubscribeUserFrom(ctx, whoId, fromWhomId)

	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return fmt.Errorf("user not subscribed: %w", ErrNotSubscribed)
		}
		return fmt.Errorf("failed to unsubscribe from user: %w", err)
	}

	return nil
}
