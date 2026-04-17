package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"code-execution/internal/config"
	"code-execution/internal/execution/domain"
)

type ExecuteSolutionUseCase struct {
	jobRepo         ExecutionJobRepository
	testResultRepo  ExecutionTestResultRepository
	templateRepo    LanguageTemplateRepository
	publisher       EventPublisher
	validator       CodeValidator
	scriptBuilder   ScriptBuilder
	sandboxRunner   SandboxRunner
	strategyFactory LanguageStrategyFactory
	clock           Clock
	uuidGenerator   UUIDGenerator
	executionCfg    config.ExecutionConfig
}

type ExecuteSolutionDependencies struct {
	JobRepo         ExecutionJobRepository
	TestResultRepo  ExecutionTestResultRepository
	TemplateRepo    LanguageTemplateRepository
	Publisher       EventPublisher
	Validator       CodeValidator
	ScriptBuilder   ScriptBuilder
	SandboxRunner   SandboxRunner
	StrategyFactory LanguageStrategyFactory
	Clock           Clock
	UUIDGenerator   UUIDGenerator
	ExecutionCfg    config.ExecutionConfig
}

func NewExecuteSolutionUseCase(deps ExecuteSolutionDependencies) *ExecuteSolutionUseCase {
	return &ExecuteSolutionUseCase{
		jobRepo:         deps.JobRepo,
		testResultRepo:  deps.TestResultRepo,
		templateRepo:    deps.TemplateRepo,
		publisher:       deps.Publisher,
		validator:       deps.Validator,
		scriptBuilder:   deps.ScriptBuilder,
		sandboxRunner:   deps.SandboxRunner,
		strategyFactory: deps.StrategyFactory,
		clock:           deps.Clock,
		uuidGenerator:   deps.UUIDGenerator,
		executionCfg:    deps.ExecutionCfg,
	}
}

func (u *ExecuteSolutionUseCase) Execute(ctx context.Context, requested SolutionExecutionRequestedEvent) error {
	if strings.TrimSpace(requested.EventID) == "" {
		return fmt.Errorf("eventId is required: %w", ErrInvalidInput)
	}

	if strings.TrimSpace(requested.Data.SolutionID) == "" {
		return fmt.Errorf("solutionId is required: %w", ErrInvalidInput)
	}

	if strings.TrimSpace(requested.Data.EntryFunctionName) == "" {
		return fmt.Errorf("entryFunctionName is required: %w", ErrInvalidInput)
	}

	language, err := domain.ParseLanguage(requested.Data.Language)
	if err != nil {
		return u.completeAsGlobalFailure(
			ctx,
			requested,
			domain.Language("unknown"),
			fmt.Sprintf("unsupported language %q", requested.Data.Language),
		)
	}

	existing, err := u.jobRepo.FindBySourceEventID(ctx, requested.EventID)
	if err != nil {
		return err
	}

	if existing != nil {
		return nil
	}

	template, err := u.templateRepo.FindActiveByLanguage(ctx, language)
	if err != nil {
		return err
	}

	if template == nil || !template.Enabled {
		return u.completeAsGlobalFailure(ctx, requested, language, "language template is not enabled")
	}

	strategy, err := u.strategyFactory.Resolve(language)
	if err != nil {
		return u.completeAsGlobalFailure(ctx, requested, language, err.Error())
	}

	if err := u.validator.Validate(language, requested.Data.Code); err != nil {
		return u.completeAsGlobalFailure(ctx, requested, language, err.Error())
	}

	now := u.clock.Now().UTC()
	job, err := domain.NewExecutionJob(
		uuidSafe(u.uuidGenerator),
		requested.EventID,
		requested.Data.SolutionID,
		language,
		template.Version,
		u.executionCfg.TimeoutMs,
		u.executionCfg.MemoryLimitMb,
		u.executionCfg.CPULimitMs,
		now,
	)
	if err != nil {
		return err
	}

	if err := u.jobRepo.Create(ctx, job); err != nil {
		if errors.Is(err, ErrAlreadyExist) {
			return nil
		}
		return err
	}

	if err := job.MarkStarted(u.clock.Now().UTC()); err != nil {
		return err
	}

	if err := u.jobRepo.Update(ctx, job); err != nil {
		return err
	}

	startedEvent := ExecutionStartedEvent{}
	startedEvent.EventID = uuidSafe(u.uuidGenerator)
	startedEvent.EventType = "SolutionExecutionStartedEvent"
	startedEvent.Timestamp = u.clock.Now().UTC()
	startedEvent.Data.SolutionID = job.SolutionID
	startedEvent.Data.ExecutionID = job.ID
	startedEvent.Data.StartedAt = *job.StartedAt

	if err := u.publisher.PublishExecutionStarted(ctx, startedEvent); err != nil {
		return err
	}

	testResults := make([]domain.ExecutionTestResult, 0, len(requested.Data.TestCases))
	totalTime := 0

	for _, testCase := range requested.Data.TestCases {
		builtScript, buildErr := u.scriptBuilder.Build(*template, BuildScriptRequest{
			UserCode:          requested.Data.Code,
			EntryFunctionName: requested.Data.EntryFunctionName,
			TestInput:         testCase.Input,
			ExpectedOutput:    testCase.ExpectedOutput,
		})
		if buildErr != nil {
			return u.finishWithGlobalFailure(ctx, &job, totalTime, fmt.Sprintf("script build failed: %v", buildErr), testResults)
		}

		executionResult, execErr := u.sandboxRunner.Execute(ctx, SandboxExecutionRequest{
			Language:       language,
			Image:          strategy.SandboxImage(),
			Entrypoint:     builtScript.Entrypoint,
			ScriptContent:  builtScript.Content,
			CompileCommand: builtScript.CompileCommand,
			RunCommand:     builtScript.RunCommand,
			TimeoutMs:      job.TimeoutMs,
			MemoryLimitMb:  job.MemoryLimitMb,
			CPULimitMs:     job.CPULimitMs,
		})
		if execErr != nil {
			return u.finishWithGlobalFailure(ctx, &job, totalTime, fmt.Sprintf("sandbox execution failed: %v", execErr), testResults)
		}

		totalTime += executionResult.ExecutionTimeMs
		actualOutput := domain.NormalizeOutput(executionResult.Output)
		expectedOutput := domain.NormalizeOutput(testCase.ExpectedOutput)
		passed := actualOutput == expectedOutput && executionResult.ExitCode == 0 && !executionResult.TimedOut

		var errorMessage *string
		if executionResult.TimedOut {
			msg := "test timeout exceeded"
			errorMessage = &msg
		} else if executionResult.ExitCode != 0 {
			msg := strings.TrimSpace(executionResult.ErrorOutput)
			if msg == "" {
				msg = fmt.Sprintf("non-zero exit code: %d", executionResult.ExitCode)
			}
			errorMessage = &msg
		}

		testResults = append(testResults, domain.ExecutionTestResult{
			ID:              uuidSafe(u.uuidGenerator),
			ExecutionJobID:  job.ID,
			TestID:          testCase.TestID,
			Passed:          passed,
			IsHidden:        testCase.IsHidden,
			InputHash:       hashInput(testCase.Input),
			ActualOutput:    actualOutput,
			ExpectedOutput:  expectedOutput,
			ErrorMessage:    errorMessage,
			ExecutionTimeMs: executionResult.ExecutionTimeMs,
		})
	}

	if err := u.testResultRepo.CreateMany(ctx, testResults); err != nil {
		return err
	}

	if err := job.MarkCompleted(u.clock.Now().UTC(), totalTime); err != nil {
		return err
	}

	if err := u.jobRepo.Update(ctx, job); err != nil {
		return err
	}

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, job, testResults)
	if err := u.publisher.PublishExecutionCompleted(ctx, completeEvent); err != nil {
		return err
	}

	return nil
}

func (u *ExecuteSolutionUseCase) completeAsGlobalFailure(
	ctx context.Context,
	requested SolutionExecutionRequestedEvent,
	language domain.Language,
	globalError string,
) error {
	now := u.clock.Now().UTC()
	templateVersion := "n/a"

	template, err := u.templateRepo.FindByLanguage(ctx, language)
	if err == nil && template != nil && template.Version != "" {
		templateVersion = template.Version
	}

	job, createErr := domain.NewExecutionJob(
		uuidSafe(u.uuidGenerator),
		requested.EventID,
		requested.Data.SolutionID,
		language,
		templateVersion,
		u.executionCfg.TimeoutMs,
		u.executionCfg.MemoryLimitMb,
		u.executionCfg.CPULimitMs,
		now,
	)
	if createErr != nil {
		return createErr
	}

	if err := u.jobRepo.Create(ctx, job); err != nil {
		if errors.Is(err, ErrAlreadyExist) {
			return nil
		}
		return err
	}

	if err := job.MarkFailed(u.clock.Now().UTC(), 0, globalError); err != nil {
		return err
	}

	if err := u.jobRepo.Update(ctx, job); err != nil {
		return err
	}

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, job, nil)
	if err := u.publisher.PublishExecutionCompleted(ctx, completeEvent); err != nil {
		return err
	}

	return nil
}

func (u *ExecuteSolutionUseCase) finishWithGlobalFailure(
	ctx context.Context,
	job *domain.ExecutionJob,
	totalTime int,
	globalError string,
	testResults []domain.ExecutionTestResult,
) error {
	if len(testResults) > 0 {
		if err := u.testResultRepo.CreateMany(ctx, testResults); err != nil {
			return err
		}
	}

	if err := job.MarkFailed(u.clock.Now().UTC(), totalTime, globalError); err != nil {
		return err
	}

	if err := u.jobRepo.Update(ctx, *job); err != nil {
		return err
	}

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, *job, testResults)
	if err := u.publisher.PublishExecutionCompleted(ctx, completeEvent); err != nil {
		return err
	}

	return nil
}

func buildCompletedEvent(
	uuidGenerator UUIDGenerator,
	clock Clock,
	job domain.ExecutionJob,
	testResults []domain.ExecutionTestResult,
) ExecutionCompletedEvent {
	event := ExecutionCompletedEvent{}
	event.EventID = uuidSafe(uuidGenerator)
	event.EventType = "SolutionExecutionCompletedEvent"
	event.Timestamp = clock.Now().UTC()
	event.Data.SolutionID = job.SolutionID
	event.Data.ExecutionID = job.ID
	event.Data.IsSuccessful = eventSuccess(job, testResults)
	if job.TotalExecutionTimeMs != nil {
		event.Data.TotalExecutionTimeMs = *job.TotalExecutionTimeMs
	}
	event.Data.GlobalError = job.GlobalError

	event.Data.TestResults = make([]ExecutionCompletedTestResult, 0, len(testResults))
	for _, result := range testResults {
		event.Data.TestResults = append(event.Data.TestResults, ExecutionCompletedTestResult{
			TestID:          result.TestID,
			Passed:          result.Passed,
			IsHidden:        result.IsHidden,
			ActualOutput:    result.ActualOutput,
			ExpectedOutput:  result.ExpectedOutput,
			ExecutionTimeMs: result.ExecutionTimeMs,
			ErrorMessage:    result.ErrorMessage,
		})
	}

	return event
}

func eventSuccess(job domain.ExecutionJob, testResults []domain.ExecutionTestResult) bool {
	if job.GlobalError != nil {
		return false
	}

	for _, result := range testResults {
		if !result.Passed {
			return false
		}
	}

	return true
}

func uuidSafe(generator UUIDGenerator) string {
	if generator == nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	value := generator.NewString()
	if strings.TrimSpace(value) == "" {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return value
}
