package domain

import "time"

type LanguageTemplate struct {
	ID             string
	Language       Language
	Version        string
	Entrypoint     string
	RunnerTemplate string
	CompileCommand *string
	RunCommand     string
	Enabled        bool
	UpdatedAt      time.Time
}
