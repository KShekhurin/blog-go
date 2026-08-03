package repositories

import (
	"context"
	"errors"
	"log/slog"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
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

func (w *userCacheWrapper) SubscribeUserTo(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	err := w.userRepo.SubscribeUserTo(ctx, subId, authId)
	if err != nil {
		return err
	}

	err = w.subsCache.SubscribeUserTo(ctx, subId, authId)
	if err != nil {
		//If addition fails we should invalidate all list
		//Cache no longer in sync with the db
		slog.ErrorContext(ctx, "UnsubscribeUserFrom", slog.Any("err", err))
		return err
	}

	return nil
}

func (w *userCacheWrapper) UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	err := w.userRepo.UnsubscribeUserFrom(ctx, subId, authId)

	if err != nil {
		return err
	}

	err = w.subsCache.UnsubscribeUserFrom(ctx, subId, authId)

	if err != nil {
		//If removal fails we should invalidate all list
		//Cache no longer in sync with the db
		slog.ErrorContext(ctx, "UnsubscribeUserFrom", slog.Any("err", err))
		return err
	}

	return err
}

func (w *userCacheWrapper) GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	subs, err := w.subsCache.GetSubs(ctx, userId)

	if errors.Is(err, errs.ErrNotFound) {
		subs, err = w.userRepo.GetSubs(ctx, userId)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		slog.ErrorContext(ctx, "GetSubs", "msg", slog.Any("err", err))
	}

	return subs, nil
}
