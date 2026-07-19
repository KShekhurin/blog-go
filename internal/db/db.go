package db

import (
	"context"

	"github.com/KShekhurin/blog-go/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Db *pgxpool.Pool
}

func (db *Database) Close() {
	db.Db.Close()
}

func Load(cfg *config.Config) (*Database, error) {
	db := &Database{}

	dbpool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)

	if err != nil {
		return nil, err
	}

	db.Db = dbpool

	return db, nil
}
