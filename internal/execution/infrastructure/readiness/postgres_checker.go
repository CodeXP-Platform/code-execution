package readiness

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresChecker struct {
	pool *pgxpool.Pool
}

func NewPostgresChecker(pool *pgxpool.Pool) PostgresChecker {
	return PostgresChecker{pool: pool}
}

func (c PostgresChecker) Name() string {
	return "postgres"
}

func (c PostgresChecker) Check(ctx context.Context) error {
	if c.pool == nil {
		return fmt.Errorf("postgres pool is nil")
	}
	if err := c.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}
	return nil
}
