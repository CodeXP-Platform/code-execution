package postgres

import (
	"context"
	"errors"
	"fmt"

	"code-execution/internal/execution/application"
	"code-execution/internal/execution/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionJobRepository struct {
	pool *pgxpool.Pool
}

func NewExecutionJobRepository(pool *pgxpool.Pool) *ExecutionJobRepository {
	return &ExecutionJobRepository{pool: pool}
}

func (r *ExecutionJobRepository) FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.ExecutionJob, error) {
	query := `SELECT id, source_event_id, solution_id, language, status, timeout_ms, memory_limit_mb, cpu_limit_ms, template_version,
		total_execution_time_ms, global_error, created_at, started_at, completed_at
	FROM execution_jobs WHERE source_event_id = $1`

	row := r.pool.QueryRow(ctx, query, sourceEventID)
	job, err := scanExecutionJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (r *ExecutionJobRepository) Create(ctx context.Context, job domain.ExecutionJob) error {
	query := `INSERT INTO execution_jobs (
		id, source_event_id, solution_id, language, status, timeout_ms, memory_limit_mb, cpu_limit_ms,
		template_version, total_execution_time_ms, global_error, created_at, started_at, completed_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`

	_, err := r.pool.Exec(
		ctx,
		query,
		job.ID,
		job.SourceEventID,
		job.SolutionID,
		string(job.Language),
		string(job.Status),
		job.TimeoutMs,
		job.MemoryLimitMb,
		job.CPULimitMs,
		job.TemplateVersion,
		job.TotalExecutionTimeMs,
		job.GlobalError,
		job.CreatedAt,
		job.StartedAt,
		job.CompletedAt,
	)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return application.ErrAlreadyExist
	}

	return fmt.Errorf("insert execution job failed: %w", err)
}

func (r *ExecutionJobRepository) Update(ctx context.Context, job domain.ExecutionJob) error {
	query := `UPDATE execution_jobs
	SET status = $2,
		total_execution_time_ms = $3,
		global_error = $4,
		started_at = $5,
		completed_at = $6
	WHERE id = $1`

	commandTag, err := r.pool.Exec(
		ctx,
		query,
		job.ID,
		string(job.Status),
		job.TotalExecutionTimeMs,
		job.GlobalError,
		job.StartedAt,
		job.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("update execution job failed: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (r *ExecutionJobRepository) FindByID(ctx context.Context, id string) (*domain.ExecutionJob, error) {
	query := `SELECT id, source_event_id, solution_id, language, status, timeout_ms, memory_limit_mb, cpu_limit_ms, template_version,
		total_execution_time_ms, global_error, created_at, started_at, completed_at
	FROM execution_jobs WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	job, err := scanExecutionJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExecutionJob(row rowScanner) (*domain.ExecutionJob, error) {
	var (
		job                  domain.ExecutionJob
		language             string
		status               string
		totalExecutionTimeMs *int
	)

	err := row.Scan(
		&job.ID,
		&job.SourceEventID,
		&job.SolutionID,
		&language,
		&status,
		&job.TimeoutMs,
		&job.MemoryLimitMb,
		&job.CPULimitMs,
		&job.TemplateVersion,
		&totalExecutionTimeMs,
		&job.GlobalError,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	parsedLanguage, err := domain.ParseLanguage(language)
	if err != nil {
		return nil, err
	}

	job.Language = parsedLanguage
	job.Status = domain.ExecutionStatus(status)
	job.TotalExecutionTimeMs = totalExecutionTimeMs

	return &job, nil
}
