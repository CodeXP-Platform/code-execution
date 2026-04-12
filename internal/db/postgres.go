package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, postgresURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(postgresURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres pool initialization failed: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	return pool, nil
}
