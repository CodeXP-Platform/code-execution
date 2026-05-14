package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
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
	log.Printf("[Execute] Iniciando procesamiento para EventID: %s, SolutionID: %s", requested.EventID, requested.Data.SolutionID)

	if strings.TrimSpace(requested.EventID) == "" {
		log.Printf("[Execute] Error: eventId is required")
		return fmt.Errorf("eventId is required: %w", ErrInvalidInput)
	}

	if strings.TrimSpace(requested.Data.SolutionID) == "" {
		log.Printf("[Execute] Error: solutionId is required")
		return fmt.Errorf("solutionId is required: %w", ErrInvalidInput)
	}

	if strings.TrimSpace(requested.Data.EntryFunctionName) == "" {
		log.Printf("[Execute] Error: entryFunctionName is required")
		return fmt.Errorf("entryFunctionName is required: %w", ErrInvalidInput)
	}

	language, err := domain.ParseLanguage(requested.Data.Language)
	if err != nil {
		log.Printf("[Execute] Error parsing language %q: %v", requested.Data.Language, err)
		return u.completeAsGlobalFailure(
			ctx,
			requested,
			domain.Language("unknown"),
			fmt.Sprintf("unsupported language %q", requested.Data.Language),
		)
	}

	existing, err := u.jobRepo.FindBySourceEventID(ctx, requested.EventID)
	if err != nil {
		log.Printf("[Execute] Error buscando job existente por EventID %s: %v", requested.EventID, err)
		return err
	}

	if existing != nil {
		log.Printf("[Execute] Job ya existente para EventID %s. Ignorando evento para mantener idempotencia.", requested.EventID)
		return nil
	}

	template, err := u.templateRepo.FindActiveByLanguage(ctx, language)
	if err != nil {
		log.Printf("[Execute] Error buscando template para lenguaje %s: %v", language, err)
		return err
	}

	if template == nil || !template.Enabled {
		log.Printf("[Execute] Error: Template para lenguaje %s no encontrado o inactivo", language)
		return u.completeAsGlobalFailure(ctx, requested, language, "language template is not enabled")
	}

	strategy, err := u.strategyFactory.Resolve(language)
	if err != nil {
		log.Printf("[Execute] Error resolviendo estrategia para lenguaje %s: %v", language, err)
		return u.completeAsGlobalFailure(ctx, requested, language, err.Error())
	}

	if err := u.validator.Validate(language, requested.Data.Code); err != nil {
		log.Printf("[Execute] Validación de código falló para EventID %s: %v", requested.EventID, err)
		return u.completeAsGlobalFailure(ctx, requested, language, err.Error())
	}
	log.Printf("[Execute] Validación de código exitosa para EventID %s", requested.EventID)

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
		log.Printf("[Execute] Error actualizando estado STARTED del job %s: %v", job.ID, err)
		return err
	}

	testResults := make([]domain.ExecutionTestResult, 0, len(requested.Data.TestCases))
	totalTime := 0

	log.Printf("[Execute] Construyendo script para JobID %s", job.ID)
	builtScript, buildErr := u.scriptBuilder.Build(*template, BuildScriptRequest{
		UserCode:          requested.Data.Code,
		EntryFunctionName: requested.Data.EntryFunctionName,
		TestCases:         requested.Data.TestCases,
	})

	// Emit EXECUTING event right before sandbox runs (or before surfacing a build failure),
	// so the consuming service always sees EXECUTING before COMPLETED/FAILED.
	startedEvent := ExecutionStartedEvent{}
	startedEvent.EventID = uuidSafe(u.uuidGenerator)
	startedEvent.EventType = "SolutionExecutionStartedEvent"
	startedEvent.Timestamp = u.clock.Now().UTC()
	startedEvent.Data.SolutionID = job.SolutionID
	startedEvent.Data.AttemptID = requested.Data.AttemptID
	startedEvent.Data.ChallengeID = requested.Data.ChallengeID
	startedEvent.Data.UserID = requested.Data.UserID
	startedEvent.Data.ExecutionID = job.ID
	startedEvent.Data.StartedAt = *job.StartedAt

	if err := u.publisher.PublishExecutionStarted(ctx, startedEvent); err != nil {
		log.Printf("[Execute] Error publicando evento Started para JobID %s: %v", job.ID, err)
		return err
	}
	log.Printf("[Execute] Evento Started publicado para JobID %s", job.ID)

	if buildErr != nil {
		log.Printf("[Execute] Error construyendo script para JobID %s: %v", job.ID, buildErr)
		return u.finishWithGlobalFailure(ctx, &job, totalTime, fmt.Sprintf("script build failed: %v", buildErr), testResults, requested)
	}

	log.Printf("[Execute] Ejecutando test suite de %d casos de prueba para JobID %s", len(requested.Data.TestCases), job.ID)

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
		log.Printf("[Execute] Error de ejecución en sandbox para JobID %s: %v", job.ID, execErr)
		return u.finishWithGlobalFailure(ctx, &job, totalTime, fmt.Sprintf("sandbox execution failed: %v", execErr), testResults, requested)
	}

	totalTime = executionResult.ExecutionTimeMs

	var globalErrorMessage *string
	if executionResult.TimedOut {
		msg := "test suite timeout exceeded"
		globalErrorMessage = &msg
	} else if executionResult.ExitCode != 0 {
		msg := strings.TrimSpace(executionResult.ErrorOutput)
		if msg == "" {
			msg = fmt.Sprintf("non-zero exit code: %d", executionResult.ExitCode)
		}
		globalErrorMessage = &msg
	}

	for i, testCase := range requested.Data.TestCases {
		testName := fmt.Sprintf("test_case_%d", i)
		passed := false

		if executionResult.TimedOut {
			passed = false
		} else if language == domain.LanguagePython {
			matched, _ := regexp.MatchString(fmt.Sprintf(`(?m)^%s\s*(?:\([^)]+\))?\s*\.\.\.\s*ok`, regexp.QuoteMeta(testName)), executionResult.ErrorOutput)
			if matched {
				passed = true
			}
		} else if language == domain.LanguageJavaScript {
			pattern := fmt.Sprintf(`(?m)(?:✔|ok\s+\d+\s+-)\s*%s\b`, regexp.QuoteMeta(testName))
			matchedOut, _ := regexp.MatchString(pattern, executionResult.Output)
			matchedErr, _ := regexp.MatchString(pattern, executionResult.ErrorOutput)
			if matchedOut || matchedErr {
				passed = true
			}
		} else {
			passed = executionResult.ExitCode == 0 && !executionResult.TimedOut
		}

		var errorMessage *string
		if !passed {
			if globalErrorMessage != nil {
				errorMessage = globalErrorMessage
			} else {
				msg := "test failed"
				errorMessage = &msg
			}
		}

		// Calculate proportional execution time per test
		testExecutionTime := totalTime / len(requested.Data.TestCases)
		if i == len(requested.Data.TestCases)-1 {
			testExecutionTime += totalTime % len(requested.Data.TestCases)
		}

		testResults = append(testResults, domain.ExecutionTestResult{
			ID:              uuidSafe(u.uuidGenerator),
			ExecutionJobID:  job.ID,
			TestID:          testCase.TestID,
			Passed:          passed,
			IsHidden:        testCase.IsHidden,
			InputHash:       hashInput(testCase.Input),
			ActualOutput:    "", // We would parse the actual output here from JSON/TAP
			ExpectedOutput:  domain.NormalizeOutput(testCase.ExpectedOutput),
			ErrorMessage:    errorMessage,
			ExecutionTimeMs: testExecutionTime,
		})
		log.Printf("[Execute] Test %s (JobID: %s) Pasó: %v", testCase.TestID, job.ID, passed)
	}

	if err := u.testResultRepo.CreateMany(ctx, testResults); err != nil {
		log.Printf("[Execute] Error guardando resultados de test para JobID %s: %v", job.ID, err)
		return err
	}

	if err := job.MarkCompleted(u.clock.Now().UTC(), totalTime); err != nil {
		return err
	}

	if err := u.jobRepo.Update(ctx, job); err != nil {
		log.Printf("[Execute] Error actualizando estado COMPLETED del job %s: %v", job.ID, err)
		return err
	}

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, job, testResults, requested)
	if err := u.publisher.PublishExecutionCompleted(ctx, completeEvent); err != nil {
		log.Printf("[Execute] Error publicando evento Completed para JobID %s: %v", job.ID, err)
		return err
	}
	log.Printf("[Execute] Flujo completado exitosamente para JobID %s", job.ID)

	return nil
}

func (u *ExecuteSolutionUseCase) completeAsGlobalFailure(
	ctx context.Context,
	requested SolutionExecutionRequestedEvent,
	language domain.Language,
	globalError string,
) error {
	log.Printf("[GlobalFailure] Marcando fallo global para EventID %s: %s", requested.EventID, globalError)
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

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, job, nil, requested)
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
	requested SolutionExecutionRequestedEvent,
) error {
	log.Printf("[FinishGlobalFailure] Fallo global durante tests para JobID %s: %s", job.ID, globalError)
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

	completeEvent := buildCompletedEvent(u.uuidGenerator, u.clock, *job, testResults, requested)
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
	requested SolutionExecutionRequestedEvent,
) ExecutionCompletedEvent {
	event := ExecutionCompletedEvent{}
	event.EventID = uuidSafe(uuidGenerator)
	event.EventType = "SolutionExecutionCompletedEvent"
	event.Timestamp = clock.Now().UTC()
	event.Data.SolutionID = job.SolutionID
	event.Data.AttemptID = requested.Data.AttemptID
	event.Data.ChallengeID = requested.Data.ChallengeID
	event.Data.UserID = requested.Data.UserID
	event.Data.Code = requested.Data.Code
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
