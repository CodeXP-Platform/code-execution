package readiness

import (
	"context"
	"fmt"

	"code-execution/internal/execution/application"
)

type SandboxChecker struct {
	runner application.SandboxRunner
}

func NewSandboxChecker(runner application.SandboxRunner) SandboxChecker {
	return SandboxChecker{runner: runner}
}

func (c SandboxChecker) Name() string {
	return "sandbox"
}

func (c SandboxChecker) Check(ctx context.Context) error {
	if c.runner == nil {
		return fmt.Errorf("sandbox runner is nil")
	}
	if err := c.runner.Healthy(ctx); err != nil {
		return fmt.Errorf("sandbox health failed: %w", err)
	}
	return nil
}
