package db

import (
	"context"

	"github.com/KShekhurin/blog-go/config"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/extra/redisotel/v9"
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

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	dbPool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, err
	}

	db.Db = dbPool

	cacheOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	cache := redis.NewClient(cacheOptions)
	if err := redisotel.InstrumentTracing(cache); err != nil {
		return nil, err
	}
	db.Cache = cache

	return db, nil
}
