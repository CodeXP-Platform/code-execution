package postgres

import (
	"context"
	"fmt"

	"code-execution/internal/execution/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionTestResultRepository struct {
	pool *pgxpool.Pool
}

func NewExecutionTestResultRepository(pool *pgxpool.Pool) *ExecutionTestResultRepository {
	return &ExecutionTestResultRepository{pool: pool}
}

func (r *ExecutionTestResultRepository) CreateMany(ctx context.Context, results []domain.ExecutionTestResult) error {
	if len(results) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `INSERT INTO execution_test_results (
		id, execution_job_id, test_id, passed, is_hidden, input_hash, actual_output, expected_output, error_message, execution_time_ms
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	for _, result := range results {
		batch.Queue(
			query,
			result.ID,
			result.ExecutionJobID,
			result.TestID,
			result.Passed,
			result.IsHidden,
			result.InputHash,
			result.ActualOutput,
			result.ExpectedOutput,
			result.ErrorMessage,
			result.ExecutionTimeMs,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range results {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert execution test result failed: %w", err)
		}
	}

	return nil
}

func (r *ExecutionTestResultRepository) FindByExecutionJobID(ctx context.Context, executionJobID string) ([]domain.ExecutionTestResult, error) {
	query := `SELECT id, execution_job_id, test_id, passed, is_hidden, input_hash, actual_output, expected_output, error_message, execution_time_ms
	FROM execution_test_results
	WHERE execution_job_id = $1
	ORDER BY test_id ASC`

	rows, err := r.pool.Query(ctx, query, executionJobID)
	if err != nil {
		return nil, fmt.Errorf("query execution test results failed: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ExecutionTestResult, 0)
	for rows.Next() {
		var item domain.ExecutionTestResult
		if err := rows.Scan(
			&item.ID,
			&item.ExecutionJobID,
			&item.TestID,
			&item.Passed,
			&item.IsHidden,
			&item.InputHash,
			&item.ActualOutput,
			&item.ExpectedOutput,
			&item.ErrorMessage,
			&item.ExecutionTimeMs,
		); err != nil {
			return nil, fmt.Errorf("scan execution test result failed: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration over execution test results failed: %w", err)
	}

	return items, nil
}
