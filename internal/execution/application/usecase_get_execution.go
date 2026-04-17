package application

import "context"

type GetExecutionUseCase struct {
	jobRepo        ExecutionJobRepository
	testResultRepo ExecutionTestResultRepository
}

func NewGetExecutionUseCase(jobRepo ExecutionJobRepository, testResultRepo ExecutionTestResultRepository) *GetExecutionUseCase {
	return &GetExecutionUseCase{jobRepo: jobRepo, testResultRepo: testResultRepo}
}

func (u *GetExecutionUseCase) Execute(ctx context.Context, executionID string) (*ExecutionDetails, error) {
	job, err := u.jobRepo.FindByID(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrNotFound
	}

	results, err := u.testResultRepo.FindByExecutionJobID(ctx, executionID)
	if err != nil {
		return nil, err
	}

	return &ExecutionDetails{Job: *job, TestResults: results}, nil
}
