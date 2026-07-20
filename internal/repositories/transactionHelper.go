package repositories

import (
	"context"

	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ExecTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(q *database.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := database.New(tx)

	err = fn(q)

	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
