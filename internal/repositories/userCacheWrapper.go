package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/errs"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

type userCacheWrapper struct {
	getSubsSF singleflight.Group
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

	exists, err := w.subsCache.FollowListExists(ctx, authId)

	if err != nil {
		//If existence check fails we should invalidate all list
		//Cache no longer in sync with the db
		slog.ErrorContext(ctx, "UnsubscribeUserFrom", slog.Any("err", err))

		return nil
	}

	if exists {
		err = w.subsCache.SubscribeUserTo(ctx, subId, authId)
		if err != nil {
			//If existence check fails we should invalidate all list
			//Cache no longer in sync with the db
			slog.ErrorContext(ctx, "SubscribeUserTo", slog.Any("err", err))
		}
		return nil
	}

	res := w.getSubsSF.DoChan(authId.String(), w.tryToCacheSubs(authId))

	select {
	case r := <-res:
		return r.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *userCacheWrapper) UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	err := w.userRepo.UnsubscribeUserFrom(ctx, subId, authId)

	if err != nil {
		return err
	}

	exists, err := w.subsCache.FollowListExists(ctx, authId)

	if err != nil {
		//If existence check fails we should invalidate all list
		//Cache no longer in sync with the db
		slog.ErrorContext(ctx, "UnsubscribeUserFrom", slog.Any("err", err))

		return nil
	}

	if exists {
		err = w.subsCache.UnsubscribeUserFrom(ctx, subId, authId)
		if err != nil {
			//If existence check fails we should invalidate all list
			//Cache no longer in sync with the db
			slog.ErrorContext(ctx, "UnsubscribeUserFrom", slog.Any("err", err))
		}
		return nil
	}

	res := w.getSubsSF.DoChan(authId.String(), w.tryToCacheSubs(authId))

	select {
	case r := <-res:
		return r.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *userCacheWrapper) tryToCacheSubs(authId uuid.UUID) func() (any, error) {
	return func() (any, error) {
		ctx := context.Background()

		//Check again in case we visited after cache.GetFollows returned error
		//but other goroutine had already refreshed cache and the channel had being closed
		subs, err := w.subsCache.GetFollows(ctx, authId)
		if err == nil {
			return subs, nil
		}

		//a failed get should not invalidate a get from repo
		if !errors.Is(err, errs.ErrNotFound) {
			slog.ErrorContext(ctx, "tryToCacheSubs: cache get", slog.Any("err", err))
		}

		subs, err = w.userRepo.GetSubs(ctx, authId)
		if err != nil {
			return nil, err
		}

		//Put data from repo to cache; a failed set doesn't invalidate the data
		err = w.subsCache.SetFollows(ctx, authId, subs)
		if err != nil {
			slog.ErrorContext(ctx, "tryToCacheSubs: cache set", slog.Any("err", err))
		}

		return subs, nil
	}
}

func (w *userCacheWrapper) GetSubs(ctx context.Context, userId uuid.UUID) ([]uuid.UUID, error) {
	subs, err := w.subsCache.GetFollows(ctx, userId)

	if err == nil {
		return subs, nil
	}

	if !errors.Is(err, errs.ErrNotFound) {
		//Cache is broken, fall back to the repo instead of returning empty data
		slog.ErrorContext(ctx, "GetFollows: cache get", slog.Any("err", err))
	}

	res := w.getSubsSF.DoChan(userId.String(), w.tryToCacheSubs(userId))

	select {
	case r := <-res:
		if r.Err != nil {
			return nil, r.Err
		}

		subs, ok := r.Val.([]uuid.UUID)
		if !ok {
			return nil, fmt.Errorf("GetFollows: unexpected singleflight result type %T", r.Val)
		}

		return subs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
