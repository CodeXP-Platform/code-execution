package application

import (
	"context"
	"time"

	"code-execution/internal/execution/domain"
)

type ExecutionJobRepository interface {
	FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.ExecutionJob, error)
	Create(ctx context.Context, job domain.ExecutionJob) error
	Update(ctx context.Context, job domain.ExecutionJob) error
	FindByID(ctx context.Context, id string) (*domain.ExecutionJob, error)
}

type ExecutionTestResultRepository interface {
	CreateMany(ctx context.Context, results []domain.ExecutionTestResult) error
	FindByExecutionJobID(ctx context.Context, executionJobID string) ([]domain.ExecutionTestResult, error)
}

type LanguageTemplateRepository interface {
	FindActiveByLanguage(ctx context.Context, language domain.Language) (*domain.LanguageTemplate, error)
	FindByLanguage(ctx context.Context, language domain.Language) (*domain.LanguageTemplate, error)
	Upsert(ctx context.Context, template domain.LanguageTemplate) error
	FindEnabled(ctx context.Context) ([]domain.LanguageTemplate, error)
	SeedDefaults(ctx context.Context) error
}

type EventPublisher interface {
	PublishExecutionStarted(ctx context.Context, event ExecutionStartedEvent) error
	PublishExecutionCompleted(ctx context.Context, event ExecutionCompletedEvent) error
}

type CodeValidator interface {
	Validate(language domain.Language, code string) error
}

type ScriptBuilder interface {
	Build(template domain.LanguageTemplate, request BuildScriptRequest) (BuiltScript, error)
}

type SandboxRunner interface {
	Execute(ctx context.Context, request SandboxExecutionRequest) (SandboxExecutionResult, error)
	Healthy(ctx context.Context) error
}

type Clock interface {
	Now() time.Time
}

type UUIDGenerator interface {
	NewString() string
}

type LanguageStrategy interface {
	Language() domain.Language
	SandboxImage() string
}

type LanguageStrategyFactory interface {
	Resolve(language domain.Language) (LanguageStrategy, error)
}

type ReadinessChecker interface {
	Check(ctx context.Context) error
}

type NamedReadinessChecker interface {
	ReadinessChecker
	Name() string
}
