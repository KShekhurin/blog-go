package repositories

import (
	"context"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/google/uuid"
)

type userCacheWrapper struct {
	userRepo  UserRepository
	subsCache cache.SubsCacher
}

func NewUserCacheWrapper(userRepo UserRepository, subsCache cache.SubsCacher) UserRepository {
	return &userCacheWrapper{
		userRepo:  userRepo,
		subsCache: subsCache,
	}
}

func (w *userCacheWrapper) FindUserById(ctx context.Context, id uuid.UUID) (*database.User, error) {
	return w.userRepo.FindUserById(ctx, id)
}

func (w *userCacheWrapper) FindUserByLogin(ctx context.Context, login string) (*database.User, error) {
	return w.userRepo.FindUserByLogin(ctx, login)
}

func (w *userCacheWrapper) FindUserByLoginOrEmail(ctx context.Context, login string, email string) (*database.User, error) {
	return w.userRepo.FindUserByLoginOrEmail(ctx, login, email)
}

func (w *userCacheWrapper) AddUser(ctx context.Context, user *database.User) error {
	return w.userRepo.AddUser(ctx, user)
}

func (w *userCacheWrapper) SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	err := w.userRepo.SubscribeUserTo(ctx, sub_id, auth_id)
	if err != nil {
		return err
	}

	err = w.subsCache.SubscribeUserTo(ctx, sub_id, auth_id)

	//TODO: cache failure != db one
	return err
}

func (w *userCacheWrapper) UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error {
	err := w.userRepo.UnsubscribeUserFrom(ctx, sub_id, auth_id)

	if err != nil {
		return err
	}

	err = w.subsCache.UnsubscribeUserFrom(ctx, sub_id, auth_id)

	//TODO: cache failure != db one
	return err
}

func (w *userCacheWrapper) GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	return w.userRepo.GetSubs(ctx, userId)
}
