package validation

import (
	"fmt"
	"strings"

	"code-execution/internal/execution/application"
	"code-execution/internal/execution/domain"
)

type Rule interface {
	Name() string
	Validate(language domain.Language, code string) error
}

type Validator struct {
	rules []Rule
}

func NewValidator(rules ...Rule) *Validator {
	return &Validator{rules: rules}
}

func (v *Validator) Validate(language domain.Language, code string) error {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return fmt.Errorf("code is required: %w", application.ErrInvalidInput)
	}

	for _, rule := range v.rules {
		if err := rule.Validate(language, code); err != nil {
			return fmt.Errorf("validation failed in %s: %w", rule.Name(), err)
		}
	}

	return nil
}

type MaxCodeLengthRule struct {
	max int
}

func NewMaxCodeLengthRule(max int) MaxCodeLengthRule {
	return MaxCodeLengthRule{max: max}
}

func (r MaxCodeLengthRule) Name() string {
	return "max_code_length"
}

func (r MaxCodeLengthRule) Validate(_ domain.Language, code string) error {
	if len(code) > r.max {
		return fmt.Errorf("code length exceeds %d characters", r.max)
	}
	return nil
}

type ForbiddenTokenRule struct {
	byLanguage map[domain.Language][]string
}

func NewForbiddenTokenRule() ForbiddenTokenRule {
	return ForbiddenTokenRule{
		byLanguage: map[domain.Language][]string{
			domain.LanguagePython: {
				"import os",
				"import subprocess",
				"__import__",
				"open(",
				"eval(",
				"exec(",
			},
			domain.LanguageJavaScript: {
				"require('fs')",
				"require(\"fs\")",
				"child_process",
				"process.exit",
				"fetch(",
				"import('node:",
			},
			domain.LanguageJava: {
				"java.net",
				"java.nio.file",
				"Runtime.getRuntime().exec",
				"ProcessBuilder",
			},
			domain.LanguageCPP: {
				"#include <fstream>",
				"#include <filesystem>",
				"system(",
				"popen(",
			},
		},
	}
}

func (r ForbiddenTokenRule) Name() string {
	return "forbidden_tokens"
}

func (r ForbiddenTokenRule) Validate(language domain.Language, code string) error {
	for _, token := range r.byLanguage[language] {
		if strings.Contains(code, token) {
			return fmt.Errorf("token %q is not allowed", token)
		}
	}
	return nil
}

var _ application.CodeValidator = (*Validator)(nil)
