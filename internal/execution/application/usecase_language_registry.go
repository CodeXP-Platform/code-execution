package application

import (
	"context"
	"fmt"
	"strings"

	"code-execution/internal/config"
	"code-execution/internal/execution/domain"
)

type LanguageRegistryUseCase struct {
	templateRepo  LanguageTemplateRepository
	executionConf config.ExecutionConfig
}

func NewLanguageRegistryUseCase(templateRepo LanguageTemplateRepository, executionConf config.ExecutionConfig) *LanguageRegistryUseCase {
	return &LanguageRegistryUseCase{templateRepo: templateRepo, executionConf: executionConf}
}

func (u *LanguageRegistryUseCase) ListEnabled(ctx context.Context) ([]LanguageInfo, error) {
	templates, err := u.templateRepo.FindEnabled(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]LanguageInfo, 0, len(templates))
	for _, template := range templates {
		items = append(items, LanguageInfo{
			Language:      template.Language,
			Version:       template.Version,
			Enabled:       template.Enabled,
			TimeoutMs:     u.executionConf.TimeoutMs,
			MemoryLimitMb: u.executionConf.MemoryLimitMb,
			CPULimitMs:    u.executionConf.CPULimitMs,
		})
	}

	return items, nil
}

func (u *LanguageRegistryUseCase) GetTemplate(ctx context.Context, rawLanguage string) (*domain.LanguageTemplate, error) {
	language, err := domain.ParseLanguage(rawLanguage)
	if err != nil {
		return nil, err
	}

	template, err := u.templateRepo.FindByLanguage(ctx, language)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrNotFound
	}

	return template, nil
}

func (u *LanguageRegistryUseCase) UpdateTemplate(
	ctx context.Context,
	rawLanguage string,
	input UpdateTemplateInput,
	clock Clock,
	uuidGen UUIDGenerator,
) (*domain.LanguageTemplate, error) {
	language, err := domain.ParseLanguage(rawLanguage)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Version) == "" || strings.TrimSpace(input.Entrypoint) == "" || strings.TrimSpace(input.RunCommand) == "" {
		return nil, fmt.Errorf("version, entrypoint and runCommand are required: %w", ErrInvalidInput)
	}

	template := domain.LanguageTemplate{
		ID:             uuidSafe(uuidGen),
		Language:       language,
		Version:        strings.TrimSpace(input.Version),
		Entrypoint:     strings.TrimSpace(input.Entrypoint),
		RunnerTemplate: input.RunnerTemplate,
		CompileCommand: input.CompileCommand,
		RunCommand:     strings.TrimSpace(input.RunCommand),
		Enabled:        input.Enabled,
		UpdatedAt:      clock.Now().UTC(),
	}

	if strings.TrimSpace(template.RunnerTemplate) == "" {
		return nil, fmt.Errorf("runnerTemplate is required: %w", ErrInvalidInput)
	}

	if err := u.templateRepo.Upsert(ctx, template); err != nil {
		return nil, err
	}

	result, err := u.templateRepo.FindByLanguage(ctx, language)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrNotFound
	}

	return result, nil
}
