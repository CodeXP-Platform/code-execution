package domain

import (
	"errors"
	"time"
)

type ExecutionJob struct {
	ID                   string
	SourceEventID        string
	SolutionID           string
	Language             Language
	Status               ExecutionStatus
	TimeoutMs            int
	MemoryLimitMb        int
	CPULimitMs           int
	TemplateVersion      string
	TotalExecutionTimeMs *int
	GlobalError          *string
	CreatedAt            time.Time
	StartedAt            *time.Time
	CompletedAt          *time.Time
}

func NewExecutionJob(
	id string,
	sourceEventID string,
	solutionID string,
	language Language,
	templateVersion string,
	timeoutMs int,
	memoryLimitMb int,
	cpuLimitMs int,
	now time.Time,
) (ExecutionJob, error) {
	if id == "" || sourceEventID == "" || solutionID == "" {
		return ExecutionJob{}, errors.New("execution job identifiers are required")
	}

	if timeoutMs <= 0 || memoryLimitMb <= 0 || cpuLimitMs <= 0 {
		return ExecutionJob{}, errors.New("execution limits must be positive")
	}

	if templateVersion == "" {
		return ExecutionJob{}, errors.New("template version is required")
	}

	return ExecutionJob{
		ID:              id,
		SourceEventID:   sourceEventID,
		SolutionID:      solutionID,
		Language:        language,
		Status:          ExecutionStatusQueued,
		TimeoutMs:       timeoutMs,
		MemoryLimitMb:   memoryLimitMb,
		CPULimitMs:      cpuLimitMs,
		TemplateVersion: templateVersion,
		CreatedAt:       now,
	}, nil
}

func (j *ExecutionJob) MarkStarted(now time.Time) error {
	next, err := j.Status.Transition(ExecutionStatusStarted)
	if err != nil {
		return err
	}

	j.Status = next
	j.StartedAt = &now
	return nil
}

func (j *ExecutionJob) MarkCompleted(now time.Time, totalExecutionTimeMs int) error {
	next, err := j.Status.Transition(ExecutionStatusCompleted)
	if err != nil {
		return err
	}

	j.Status = next
	j.TotalExecutionTimeMs = &totalExecutionTimeMs
	j.CompletedAt = &now
	j.GlobalError = nil
	return nil
}

func (j *ExecutionJob) MarkFailed(now time.Time, totalExecutionTimeMs int, globalError string) error {
	next, err := j.Status.Transition(ExecutionStatusFailed)
	if err != nil {
		return err
	}

	j.Status = next
	j.TotalExecutionTimeMs = &totalExecutionTimeMs
	j.CompletedAt = &now
	j.GlobalError = &globalError
	return nil
}
