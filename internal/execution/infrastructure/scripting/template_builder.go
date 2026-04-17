package scripting

import (
	"fmt"
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

	// Inyectar el helper parseArgs que el template asume que existe ("inyectado por el runner")
	helper := getParseArgsHelper(template.Language)
	fullUserCode := helper + "\n" + request.UserCode

	replaced := template.RunnerTemplate
	replaced = strings.ReplaceAll(replaced, "{{USER_CODE}}", fullUserCode)

	// Escapar inputs según el lenguaje para evitar errores de sintaxis en el código generado
	safeInput := escapeString(template.Language, request.TestInput)
	safeExpected := escapeString(template.Language, request.ExpectedOutput)

	replaced = strings.ReplaceAll(replaced, "{{TEST_INPUT}}", safeInput)
	replaced = strings.ReplaceAll(replaced, "{{EXPECTED_OUTPUT}}", safeExpected)
	replaced = strings.ReplaceAll(replaced, "{{ENTRY_FUNCTION_NAME}}", entryFunction)

	return application.BuiltScript{
		Entrypoint:     template.Entrypoint,
		Content:        replaced,
		CompileCommand: template.CompileCommand,
		RunCommand:     template.RunCommand,
	}, nil
}

func getParseArgsHelper(lang domain.Language) string {
	switch lang {
	case domain.LanguagePython:
		return `
def parse_args(input_str):
    if not input_str:
        return []
    parts = input_str.split(",")
    parsed = []
    for p in parts:
        p = p.strip()
        if p.isdigit() or (p.startswith('-') and p[1:].isdigit()):
            parsed.append(int(p))
        elif p.lower() == 'true':
            parsed.append(True)
        elif p.lower() == 'false':
            parsed.append(False)
        else:
            try:
                parsed.append(float(p))
            except ValueError:
                parsed.append(p)
    return parsed
`
	case domain.LanguageJavaScript:
		return `
function parseArgs(inputStr) {
    if (!inputStr) return [];
    return inputStr.split(",").map(p => {
        p = p.trim();
        if (!isNaN(p) && p !== "") return Number(p);
        if (p.toLowerCase() === 'true') return true;
        if (p.toLowerCase() === 'false') return false;
        return p;
    });
}
`
	case domain.LanguageJava:
		return `
	static Object[] parseArgs(String inputStr) {
		if (inputStr == null || inputStr.trim().isEmpty()) return new Object[0];
		String[] parts = inputStr.split(",");
		Object[] parsed = new Object[parts.length];
		for (int i = 0; i < parts.length; i++) {
			String p = parts[i].trim();
			if (p.equalsIgnoreCase("true")) parsed[i] = true;
			else if (p.equalsIgnoreCase("false")) parsed[i] = false;
			else {
				try {
					parsed[i] = Integer.parseInt(p);
				} catch (NumberFormatException e1) {
					try {
						parsed[i] = Double.parseDouble(p);
					} catch (NumberFormatException e2) {
						parsed[i] = p;
					}
				}
			}
		}
		return parsed;
	}
`
	case domain.LanguageCPP:
		return `
std::vector<std::string> parseArgs(const std::string& inputStr) {
    std::vector<std::string> result;
    if (inputStr.empty()) return result;
    size_t start = 0;
    size_t end = inputStr.find(',');
    while (end != std::string::npos) {
        result.push_back(inputStr.substr(start, end - start));
        start = end + 1;
        end = inputStr.find(',', start);
    }
    result.push_back(inputStr.substr(start));
    return result;
}
`
	}
	return ""
}

func escapeString(lang domain.Language, val string) string {
	if lang == domain.LanguageCPP {
		// C++ usa raw string literals R"(...)"
		return val
	}

	escaped := strings.ReplaceAll(val, "\\", "\\\\")

	switch lang {
	case domain.LanguageJavaScript:
		// JS usa template literals `...`
		escaped = strings.ReplaceAll(escaped, "`", "\\`")
		escaped = strings.ReplaceAll(escaped, "$", "\\$")
	case domain.LanguageJava:
		// Java usa comillas dobles "..."
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
	case domain.LanguagePython:
		// Python usa triple comillas """..."""
		escaped = strings.ReplaceAll(escaped, "\"\"\"", "\\\"\\\"\\\"")
	}

	return escaped
}

var _ application.ScriptBuilder = (*TemplateScriptBuilder)(nil)
