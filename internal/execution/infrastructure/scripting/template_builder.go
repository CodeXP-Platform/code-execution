package scripting

import (
	"fmt"
	"strconv"
	"strings"

	"code-execution/internal/execution/application"
	"code-execution/internal/execution/domain"
)

type TemplateScriptBuilder struct{}

func NewTemplateScriptBuilder() TemplateScriptBuilder {
	return TemplateScriptBuilder{}
}

func (TemplateScriptBuilder) Build(template domain.LanguageTemplate, request application.BuildScriptRequest) (application.BuiltScript, error) {
	entryFunction := strings.TrimSpace(request.EntryFunctionName)
	if entryFunction == "" {
		return application.BuiltScript{}, fmt.Errorf("entry function name is required: %w", application.ErrInvalidInput)
	}

	functionCall, err := buildFunctionCall(template.Language, entryFunction, request.TestInput)
	if err != nil {
		return application.BuiltScript{}, err
	}

	replaced := template.RunnerTemplate
	replaced = strings.ReplaceAll(replaced, "{{USER_CODE}}", request.UserCode)
	replaced = strings.ReplaceAll(replaced, "{{TEST_INPUT}}", request.TestInput)
	replaced = strings.ReplaceAll(replaced, "{{EXPECTED_OUTPUT}}", request.ExpectedOutput)
	replaced = strings.ReplaceAll(replaced, "{{ENTRY_FUNCTION_NAME}}", entryFunction)
	replaced = strings.ReplaceAll(replaced, "{{FUNCTION_CALL}}", functionCall)

	return application.BuiltScript{
		Entrypoint:     template.Entrypoint,
		Content:        replaced,
		CompileCommand: template.CompileCommand,
		RunCommand:     template.RunCommand,
	}, nil
}

func buildFunctionCall(language domain.Language, functionName string, rawInput string) (string, error) {
	args, err := parseArguments(rawInput)
	if err != nil {
		return "", err
	}

	serialized := make([]string, 0, len(args))
	for _, arg := range args {
		serialized = append(serialized, serializeArgument(language, arg))
	}

	return fmt.Sprintf("%s(%s)", functionName, strings.Join(serialized, ", ")), nil
}

func parseArguments(rawInput string) ([]string, error) {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed == "" {
		return []string{}, nil
	}

	parts := strings.Split(trimmed, ",")
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		args = append(args, item)
	}

	return args, nil
}

func serializeArgument(language domain.Language, raw string) string {
	lower := strings.ToLower(raw)
	if lower == "true" || lower == "false" {
		if language == domain.LanguagePython {
			if lower == "true" {
				return "True"
			}
			return "False"
		}
		return lower
	}

	if _, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return raw
	}

	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return raw
	}

	return strconv.Quote(raw)
}

var _ application.ScriptBuilder = (*TemplateScriptBuilder)(nil)
