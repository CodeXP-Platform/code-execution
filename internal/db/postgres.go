package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, postgresURL string) (*pgxpool.Pool, error) {
	log.Printf("Iniciando conexión a base de datos Postgres...")
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

	log.Printf("Verificando conectividad con Postgres (Ping)...")
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	log.Printf("Conexión a Postgres establecida con éxito")
	return pool, nil
}
