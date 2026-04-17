package languages

import "code-execution/internal/execution/domain"

type Strategy struct {
	language     domain.Language
	sandboxImage string
}

func NewStrategy(language domain.Language, sandboxImage string) Strategy {
	return Strategy{language: language, sandboxImage: sandboxImage}
}

func (s Strategy) Language() domain.Language {
	return s.language
}

func (s Strategy) SandboxImage() string {
	return s.sandboxImage
}
