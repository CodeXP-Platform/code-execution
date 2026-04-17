package domain

import (
	"fmt"
	"strings"
)

type Language string

const (
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageJava       Language = "java"
	LanguageCPP        Language = "cpp"
)

func ParseLanguage(raw string) (Language, error) {
	value := Language(strings.ToLower(strings.TrimSpace(raw)))

	switch value {
	case LanguagePython, LanguageJavaScript, LanguageJava, LanguageCPP:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported language %q", raw)
	}
}
