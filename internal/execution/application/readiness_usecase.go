package application

import (
	"context"
	"fmt"
	"time"
)

type ReadinessStatus struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
	Errors       map[string]string `json:"errors,omitempty"`
}

type ReadinessUseCase struct {
	checkers []NamedReadinessChecker
}

func NewReadinessUseCase(checkers ...NamedReadinessChecker) *ReadinessUseCase {
	return &ReadinessUseCase{checkers: checkers}
}

func (u *ReadinessUseCase) Execute(ctx context.Context, timeout time.Duration) ReadinessStatus {
	status := ReadinessStatus{
		Status:       "UP",
		Dependencies: make(map[string]string, len(u.checkers)),
		Errors:       map[string]string{},
	}

	for _, checker := range u.checkers {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		err := checker.Check(checkCtx)
		cancel()

		if err != nil {
			status.Status = "DOWN"
			status.Dependencies[checker.Name()] = "DOWN"
			status.Errors[checker.Name()] = err.Error()
			continue
		}

		status.Dependencies[checker.Name()] = "UP"
	}

	if len(status.Errors) == 0 {
		status.Errors = nil
	}

	return status
}

func (s ReadinessStatus) HTTPCode() int {
	if s.Status == "UP" {
		return 200
	}
	return 503
}

func (s ReadinessStatus) Error() error {
	if s.Status == "UP" {
		return nil
	}
	return fmt.Errorf("readiness status is %s", s.Status)
}
