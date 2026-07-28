//go:build integration

package migrations

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/moby/moby/api/types/network"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redisCont "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed *.sql
var migrations embed.FS

const (
	PostgresDbName   = "blog_db"
	PostgresUser     = "postgres"
	PostgresPassword = "postgres"
)

func SetupPostgres(ctx context.Context, t testing.TB) *pgxpool.Pool {
	t.Helper()

	postgresContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(PostgresDbName),
		postgres.WithUsername(PostgresUser),
		postgres.WithPassword(PostgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForSQL(
				"5432/tcp",
				"pgx",
				func(host string, port network.Port) string {
					return fmt.Sprintf(
						"postgres://%s:%s@%s:%s/%s?sslmode=disable",
						PostgresUser, PostgresPassword, host, port.Port(), PostgresDbName,
					)
				},
			).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set dialect: %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	return pool
}

func SetupRedis(ctx context.Context, t testing.TB) *redis.Client {
	t.Helper()

	redisContainer, err := redisCont.Run(ctx, "redis:8-alpine")

	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	connStr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	options, err := redis.ParseURL(connStr)
	if err != nil {
		t.Fatalf("failed to parse redis URL: %v", err)
	}

	client := redis.NewClient(options)

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Fatalf("failed to ping redis: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate redis container: %v", err)
		}
	})

	return client
}
