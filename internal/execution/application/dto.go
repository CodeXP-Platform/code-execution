package application

import (
	"time"

	"code-execution/internal/execution/domain"
)

type TestCaseInput struct {
	TestID         string `json:"testId"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
	IsHidden       bool   `json:"isHidden"`
}

type SolutionExecutionRequestedEvent struct {
	EventID   string    `json:"eventId"`
	EventType string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		SolutionID        string          `json:"solutionId"`
		Language          string          `json:"language"`
		EntryFunctionName string          `json:"entryFunctionName"`
		Code              string          `json:"code"`
		TestCases         []TestCaseInput `json:"testCases"`
	} `json:"data"`
}

type ExecutionStartedEvent struct {
	EventID   string    `json:"eventId"`
	EventType string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		SolutionID  string    `json:"solutionId"`
		ExecutionID string    `json:"executionId"`
		StartedAt   time.Time `json:"startedAt"`
	} `json:"data"`
}

type ExecutionCompletedTestResult struct {
	TestID          string  `json:"testId"`
	Passed          bool    `json:"passed"`
	IsHidden        bool    `json:"isHidden"`
	ActualOutput    string  `json:"actualOutput"`
	ExpectedOutput  string  `json:"expectedOutput"`
	ExecutionTimeMs int     `json:"executionTimeMs"`
	ErrorMessage    *string `json:"errorMessage"`
}

type ExecutionCompletedEvent struct {
	EventID   string    `json:"eventId"`
	EventType string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		SolutionID           string                         `json:"solutionId"`
		ExecutionID          string                         `json:"executionId"`
		IsSuccessful         bool                           `json:"isSuccessful"`
		TotalExecutionTimeMs int                            `json:"totalExecutionTimeMs"`
		GlobalError          *string                        `json:"globalError"`
		TestResults          []ExecutionCompletedTestResult `json:"testResults"`
	} `json:"data"`
}

type BuildScriptRequest struct {
	UserCode          string
	EntryFunctionName string
	TestCases         []TestCaseInput
}

type BuiltScript struct {
	Entrypoint     string
	Content        string
	CompileCommand *string
	RunCommand     string
}

type SandboxExecutionRequest struct {
	Language       domain.Language
	Image          string
	Entrypoint     string
	ScriptContent  string
	CompileCommand *string
	RunCommand     string
	TimeoutMs      int
	MemoryLimitMb  int
	CPULimitMs     int
}

type SandboxExecutionResult struct {
	Output          string
	ErrorOutput     string
	ExecutionTimeMs int
	TimedOut        bool
	ExitCode        int
}

type ExecutionDetails struct {
	Job         domain.ExecutionJob
	TestResults []domain.ExecutionTestResult
}

type UpdateTemplateInput struct {
	Version        string  `json:"version" binding:"required"`
	Entrypoint     string  `json:"entrypoint" binding:"required"`
	RunnerTemplate string  `json:"runnerTemplate" binding:"required"`
	CompileCommand *string `json:"compileCommand"`
	RunCommand     string  `json:"runCommand" binding:"required"`
	Enabled        bool    `json:"enabled"`
}

type LanguageInfo struct {
	Language      domain.Language `json:"language"`
	Version       string          `json:"version"`
	Enabled       bool            `json:"enabled"`
	TimeoutMs     int             `json:"timeoutMs"`
	MemoryLimitMb int             `json:"memoryLimitMb"`
	CPULimitMs    int             `json:"cpuLimitMs"`
}
