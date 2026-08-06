package db

import (
	"context"

	"github.com/KShekhurin/blog-go/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	Db    *pgxpool.Pool
	Cache *redis.Client
}

func (db *Database) Close() error {
	db.Db.Close()
	return db.Cache.Close()
}

func Load(cfg *config.Config) (*Database, error) {
	db := &Database{}

	dbpool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	db.Db = dbpool

	cacheOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	cache := redis.NewClient(cacheOptions)
	db.Cache = cache

	return db, nil
}
