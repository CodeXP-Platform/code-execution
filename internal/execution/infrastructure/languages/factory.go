package languages

import (
	"fmt"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"
	"code-execution/internal/execution/domain"
)

type StrategyFactory struct {
	strategies map[domain.Language]application.LanguageStrategy
}

func NewStrategyFactory(sandboxCfg config.SandboxConfig) *StrategyFactory {
	strategies := map[domain.Language]application.LanguageStrategy{
		domain.LanguagePython:     NewStrategy(domain.LanguagePython, sandboxCfg.PythonImage),
		domain.LanguageJavaScript: NewStrategy(domain.LanguageJavaScript, sandboxCfg.JavaScriptImage),
		domain.LanguageJava:       NewStrategy(domain.LanguageJava, sandboxCfg.JavaImage),
		domain.LanguageCPP:        NewStrategy(domain.LanguageCPP, sandboxCfg.CPPImage),
	}

	return &StrategyFactory{strategies: strategies}
}

func (f *StrategyFactory) Resolve(language domain.Language) (application.LanguageStrategy, error) {
	strategy, ok := f.strategies[language]
	if !ok {
		return nil, fmt.Errorf("strategy not found for language %s", language)
	}
	return strategy, nil
}
