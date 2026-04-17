package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS execution_jobs (
			id TEXT PRIMARY KEY,
			source_event_id TEXT NOT NULL UNIQUE,
			solution_id TEXT NOT NULL,
			language TEXT NOT NULL,
			status TEXT NOT NULL,
			timeout_ms INT NOT NULL,
			memory_limit_mb INT NOT NULL,
			cpu_limit_ms INT NOT NULL,
			template_version TEXT NOT NULL,
			total_execution_time_ms INT NULL,
			global_error TEXT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			started_at TIMESTAMPTZ NULL,
			completed_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE IF NOT EXISTS execution_test_results (
			id TEXT PRIMARY KEY,
			execution_job_id TEXT NOT NULL REFERENCES execution_jobs(id) ON DELETE CASCADE,
			test_id TEXT NOT NULL,
			passed BOOLEAN NOT NULL,
			is_hidden BOOLEAN NOT NULL,
			input_hash TEXT NOT NULL,
			actual_output TEXT NOT NULL,
			expected_output TEXT NOT NULL,
			error_message TEXT NULL,
			execution_time_ms INT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_test_results_execution_job_id
		 ON execution_test_results(execution_job_id)`,
		`CREATE TABLE IF NOT EXISTS language_templates (
			id TEXT PRIMARY KEY,
			language TEXT NOT NULL UNIQUE,
			version TEXT NOT NULL,
			entrypoint TEXT NOT NULL,
			runner_template TEXT NOT NULL,
			compile_command TEXT NULL,
			run_command TEXT NOT NULL,
			enabled BOOLEAN NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	}

	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("schema statement failed: %w", err)
		}
	}

	return nil
}
