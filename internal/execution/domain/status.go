package domain

import "fmt"

type ExecutionStatus string

const (
	ExecutionStatusQueued    ExecutionStatus = "QUEUED"
	ExecutionStatusStarted   ExecutionStatus = "STARTED"
	ExecutionStatusCompleted ExecutionStatus = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatus = "FAILED"
)

func (s ExecutionStatus) CanTransitionTo(next ExecutionStatus) bool {
	switch s {
	case ExecutionStatusQueued:
		return next == ExecutionStatusStarted || next == ExecutionStatusFailed
	case ExecutionStatusStarted:
		return next == ExecutionStatusCompleted || next == ExecutionStatusFailed
	case ExecutionStatusCompleted, ExecutionStatusFailed:
		return false
	default:
		return false
	}
}

func (s ExecutionStatus) Transition(next ExecutionStatus) (ExecutionStatus, error) {
	if !s.CanTransitionTo(next) {
		return s, fmt.Errorf("invalid status transition from %s to %s", s, next)
	}
	return next, nil
}
