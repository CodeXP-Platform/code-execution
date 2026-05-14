package module

import (
	"context"
	"fmt"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"
	"code-execution/internal/execution/infrastructure/languages"
	executionrabbit "code-execution/internal/execution/infrastructure/messaging/rabbitmq"
	"code-execution/internal/execution/infrastructure/persistence/postgres"
	"code-execution/internal/execution/infrastructure/readiness"
	"code-execution/internal/execution/infrastructure/sandbox"
	"code-execution/internal/execution/infrastructure/scripting"
	"code-execution/internal/execution/infrastructure/system"
	"code-execution/internal/execution/infrastructure/validation"
	executionhttp "code-execution/internal/execution/interfaces/http"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Module struct {
	HTTPHandler *executionhttp.Handler
	consumers   []application.EventConsumer
}

func New(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, rabbitConn *amqp.Connection) (*Module, error) {
	if err := postgres.EnsureSchema(ctx, pool); err != nil {
		return nil, fmt.Errorf("ensure execution schema failed: %w", err)
	}

	jobRepo := postgres.NewExecutionJobRepository(pool)
	testResultRepo := postgres.NewExecutionTestResultRepository(pool)
	templateRepo := postgres.NewLanguageTemplateRepository(pool)

	if err := templateRepo.SeedDefaults(ctx); err != nil {
		return nil, fmt.Errorf("seed language templates failed: %w", err)
	}

	clock := system.NewClock()
	uuidGenerator := system.NewUUIDGenerator()

	strategyFactory := languages.NewStrategyFactory(cfg.Execution.Sandbox)
	scriptBuilder := scripting.NewTemplateScriptBuilder()
	sandboxRunner := sandbox.NewDockerRunner(cfg.Execution.Sandbox)

	if err := sandboxRunner.EnsureImages(ctx); err != nil {
		return nil, fmt.Errorf("docker image validation failed: %w", err)
	}

	validator := validation.NewValidator(
		validation.NewMaxCodeLengthRule(25_000),
		validation.NewForbiddenTokenRule(),
	)
	publisher := executionrabbit.NewEventPublisher(rabbitConn, cfg.Messaging)

	executeUseCase := application.NewExecuteSolutionUseCase(application.ExecuteSolutionDependencies{
		JobRepo:         jobRepo,
		TestResultRepo:  testResultRepo,
		TemplateRepo:    templateRepo,
		Publisher:       publisher,
		Validator:       validator,
		ScriptBuilder:   scriptBuilder,
		SandboxRunner:   sandboxRunner,
		StrategyFactory: strategyFactory,
		Clock:           clock,
		UUIDGenerator:   uuidGenerator,
		ExecutionCfg:    cfg.Execution,
	})

	languageUseCase := application.NewLanguageRegistryUseCase(templateRepo, cfg.Execution)
	getExecutionUseCase := application.NewGetExecutionUseCase(jobRepo, testResultRepo)

	readinessUseCase := application.NewReadinessUseCase(
		readiness.NewPostgresChecker(pool),
		readiness.NewRabbitMQChecker(rabbitConn),
		readiness.NewSandboxChecker(sandboxRunner),
	)

	handler := executionhttp.NewHandler(
		cfg,
		languageUseCase,
		getExecutionUseCase,
		readinessUseCase,
		clock,
		uuidGenerator,
	)

	consumer := executionrabbit.NewRequestedConsumer(rabbitConn, cfg.Messaging, executeUseCase)

	return &Module{
		HTTPHandler: handler,
		consumers:   []application.EventConsumer{consumer},
	}, nil
}

func (m *Module) StartConsumers(ctx context.Context) error {
	for _, consumer := range m.consumers {
		if err := consumer.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}
