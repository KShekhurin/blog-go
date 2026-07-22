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
	SubscribeTo(ctx context.Context, whoId uuid.UUID, toWhomId uuid.UUID) error
	UnsubscribeFrom(ctx context.Context, whoId uuid.UUID, fromWhomId uuid.UUID) error
}

var (
	ErrorUserExist         = errors.New("user already exists")
	ErrorAlreadySubscribed = errors.New("user already subscribed")
	ErrorIsNotSubscribed   = errors.New("user isn't subscribed")
	ErrorBadPayload        = errors.New("bad payload")
	ErrorInvalidPassword   = errors.New("invalid password")
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

	return usr, err
}

func (s *userService) CreateUser(ctx context.Context, userInfo *webModels.UserRegisterInfo) (*database.User, error) {
	_, err := s.GetUserByLoginOrEmail(ctx, userInfo.Login, userInfo.Email)

	if err == nil {
		return nil, ErrorUserExist
	}
	if !errors.Is(err, repositories.ErrorDoesNotExist) {
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
		if repositories.IsUniqueViolation(err) {
			return nil, ErrorUserExist
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return newUser, nil
}

func (s *userService) SubscribeTo(ctx context.Context, whoId uuid.UUID, toWhomId uuid.UUID) error {
	err := s.userRepo.SubscribeUserTo(ctx, whoId, toWhomId)

	if err != nil {
		if repositories.IsUniqueViolation(err) {
			return ErrorAlreadySubscribed
		}
		return fmt.Errorf("failed to subscribe to user: %w", err)
	}

	return nil
}

func (s *userService) UnsubscribeFrom(ctx context.Context, whoId uuid.UUID, fromWhomId uuid.UUID) error {
	err := s.userRepo.UnsubscribeUserFrom(ctx, whoId, fromWhomId)

	if err != nil {
		//TODO: fix
		if repositories.IsUniqueViolation(err) {
			return ErrorIsNotSubscribed
		}
		return fmt.Errorf("failed to unsubscribe from user: %w", err)
	}

	return nil
}
