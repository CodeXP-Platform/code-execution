package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type TestCase struct {
	TestID         string
	Input          string
	ExpectedOutput string
	IsHidden       bool
}

type TestResult struct {
	TestID          string  `json:"testId"`
	Passed          bool    `json:"passed"`
	IsHidden        bool    `json:"isHidden"`
	ActualOutput    string  `json:"actualOutput"`
	ExpectedOutput  string  `json:"expectedOutput"`
	ExecutionTimeMs int64   `json:"executionTimeMs"`
	ErrorMessage    *string `json:"errorMessage"`
}

type ExecutionResult struct {
	IsSuccessful         bool
	TotalExecutionTimeMs int64
	GlobalError          *string
	TestResults          []TestResult
}

func ExecuteCode(ctx context.Context, language, code string, testCases []TestCase) ExecutionResult {
	startGlobal := time.Now()
	res := ExecutionResult{
		IsSuccessful: true,
		TestResults:  make([]TestResult, 0, len(testCases)),
	}

	// 1. Determine image and command based on language
	switch strings.ToLower(language) {
	case "python", "python3":
	case "javascript", "node", "nodejs":
	default:
		errMsg := fmt.Sprintf("Unsupported language: %s", language)
		res.GlobalError = &errMsg
		res.IsSuccessful = false
		return res
	}

	// 2. Ejecutar test cases
	for _, tc := range testCases {
		testStart := time.Now()

		// Run Docker container with the mounted code
		// Using DinD approach: we map the host directory (tempDir) to /code inside the container
		// If running inside docker, this volume mount might require sharing the same host volume
		// To make it robust for DinD, passing the script via stdin to the interpreter is safer
		// if we can't guarantee volume mounts work as expected in DinD without specific host configs.
		// For now, let's use the standard `docker run` with -i and pass input via stdin.

		dockerArgs := []string{
			"run", "--rm", "-i",
			"--net", "none", // Security: disable network
			"--memory", "128m", // Security: limit memory
		}

		// Instead of volume mounts which can be tricky in DinD without knowing paths,
		// let's pass the code as a string to the interpreter directly if possible,
		// or use a different approach. Let's try volume mount first, but since tempDir is on host
		// it should work if code-execution is running on host. If it's in DinD, volume paths need to match host paths.
		// Let's use `docker run -i <image> <command> < file.py` or similar. Actually we can do `docker run -i python:3.9-alpine python -c "<code>"`.

		var cmd *exec.Cmd
		if strings.ToLower(language) == "python" || strings.ToLower(language) == "python3" {
			dockerArgs = append(dockerArgs, "python:3.9-alpine", "python", "-c", code)
		} else if strings.ToLower(language) == "javascript" || strings.ToLower(language) == "node" || strings.ToLower(language) == "nodejs" {
			dockerArgs = append(dockerArgs, "node:18-alpine", "node", "-e", code)
		}

		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)

		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		cmd.Stdin = strings.NewReader(tc.Input)

		err := cmd.Run()
		execTimeMs := time.Since(testStart).Milliseconds()

		actualOutput := strings.TrimSpace(stdoutBuf.String())
		expectedOutput := strings.TrimSpace(tc.ExpectedOutput)

		passed := err == nil && actualOutput == expectedOutput
		if !passed {
			res.IsSuccessful = false
		}

		var errorMessage *string
		if err != nil {
			errStr := strings.TrimSpace(stderrBuf.String())
			if errStr == "" {
				errStr = err.Error()
			}
			errorMessage = &errStr
		}

		res.TestResults = append(res.TestResults, TestResult{
			TestID:          tc.TestID,
			Passed:          passed,
			IsHidden:        tc.IsHidden,
			ActualOutput:    actualOutput,
			ExpectedOutput:  expectedOutput,
			ExecutionTimeMs: execTimeMs,
			ErrorMessage:    errorMessage,
		})
	}

	res.TotalExecutionTimeMs = time.Since(startGlobal).Milliseconds()
	return res
}
