package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"
)

type DockerRunner struct {
	cfg config.SandboxConfig
}

func NewDockerRunner(cfg config.SandboxConfig) *DockerRunner {
	return &DockerRunner{cfg: cfg}
}

func (r *DockerRunner) Execute(ctx context.Context, request application.SandboxExecutionRequest) (application.SandboxExecutionResult, error) {
	workdir, err := os.MkdirTemp("", "codeexec-*")
	if err != nil {
		return application.SandboxExecutionResult{}, fmt.Errorf("create temp sandbox dir failed: %w", err)
	}
	defer os.RemoveAll(workdir)

	entrypointPath := filepath.Join(workdir, request.Entrypoint)
	if err := os.WriteFile(entrypointPath, []byte(request.ScriptContent), 0o600); err != nil {
		return application.SandboxExecutionResult{}, fmt.Errorf("write script failed: %w", err)
	}

	executionStart := time.Now()

	if request.CompileCommand != nil && strings.TrimSpace(*request.CompileCommand) != "" {
		compileResult, compileErr := r.runDockerCommand(ctx, workdir, request.Image, *request.CompileCommand, request.MemoryLimitMb, request.CPULimitMs, request.TimeoutMs)
		if compileErr != nil {
			return application.SandboxExecutionResult{}, compileErr
		}
		if compileResult.ExitCode != 0 {
			return application.SandboxExecutionResult{
				Output:          compileResult.Output,
				ErrorOutput:     compileResult.ErrorOutput,
				ExecutionTimeMs: int(time.Since(executionStart).Milliseconds()),
				TimedOut:        compileResult.TimedOut,
				ExitCode:        compileResult.ExitCode,
			}, nil
		}
	}

	runResult, runErr := r.runDockerCommand(ctx, workdir, request.Image, request.RunCommand, request.MemoryLimitMb, request.CPULimitMs, request.TimeoutMs)
	if runErr != nil {
		return application.SandboxExecutionResult{}, runErr
	}

	runResult.ExecutionTimeMs = int(time.Since(executionStart).Milliseconds())
	return runResult, nil
}

func (r *DockerRunner) Healthy(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, r.cfg.DockerBinary, "version", "--format", "{{.Server.Version}}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker runtime not ready: %w, stderr=%s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// EnsureImages checks that all configured language images are available locally,
// pulling any that are missing. Called once at startup to prevent runtime failures.
func (r *DockerRunner) EnsureImages(ctx context.Context) error {
	images := []string{
		r.cfg.PythonImage,
		r.cfg.JavaScriptImage,
		r.cfg.JavaImage,
		r.cfg.CPPImage,
	}

	for _, image := range images {
		if err := r.ensureImage(ctx, image); err != nil {
			return err
		}
	}
	return nil
}

func (r *DockerRunner) ensureImage(ctx context.Context, image string) error {
	checkCmd := exec.CommandContext(ctx, r.cfg.DockerBinary, "image", "inspect", "--format", "{{.Id}}", image)
	if err := checkCmd.Run(); err == nil {
		log.Printf("[DockerRunner] Image already present: %s", image)
		return nil
	}

	log.Printf("[DockerRunner] Image not found locally, pulling: %s", image)
	pullCmd := exec.CommandContext(ctx, r.cfg.DockerBinary, "pull", image)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("failed to pull docker image %s: %w", image, err)
	}
	log.Printf("[DockerRunner] Image pulled successfully: %s", image)
	return nil
}

func (r *DockerRunner) runDockerCommand(
	ctx context.Context,
	workdir string,
	image string,
	command string,
	memoryLimitMb int,
	cpuLimitMs int,
	timeoutMs int,
) (application.SandboxExecutionResult, error) {
	log.Printf("[DockerRunner] Iniciando contenedor con imagen: %s", image)
	log.Printf("[DockerRunner] Comando interno a ejecutar: %s", command)
	dockerArgs := []string{
		"run",
		"--rm",
		"--network",
		"none",
		"--memory",
		fmt.Sprintf("%dm", memoryLimitMb),
		"--cpus",
		cpuQuota(cpuLimitMs),
		"--pids-limit",
		"64",
		"--read-only",
		"-v",
		fmt.Sprintf("%s:/workspace", filepath.Clean(workdir)),
		"-w",
		"/workspace",
		image,
		"/bin/sh",
		"-lc",
		command,
	}

	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(timedCtx, r.cfg.DockerBinary, dockerArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[DockerRunner] Ejecutando comando docker: %v", cmd.Args)
	err := cmd.Run()

	log.Printf("[DockerRunner] Contenedor finalizado. Salida estándar (stdout): %q", stdout.String())
	if stderr.Len() > 0 {
		log.Printf("[DockerRunner] Salida de error (stderr): %q", stderr.String())
	}

	result := application.SandboxExecutionResult{
		Output:      stdout.String(),
		ErrorOutput: stderr.String(),
		TimedOut:    timedCtx.Err() == context.DeadlineExceeded,
		ExitCode:    exitCode(err),
	}

	if err != nil {
		if timedCtx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = 124
			return result, nil
		}
		if result.ExitCode == -1 {
			return application.SandboxExecutionResult{}, fmt.Errorf("docker command failed: %w, stderr=%s", err, strings.TrimSpace(stderr.String()))
		}
	}

	return result, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if ok := errorAs(err, &exitErr); ok {
		return exitErr.ExitCode()
	}

	return -1
}

func cpuQuota(cpuLimitMs int) string {
	if cpuLimitMs <= 0 {
		return "0.50"
	}

	value := float64(cpuLimitMs) / 1000.0
	if value < 0.1 {
		value = 0.1
	}
	if value > 2.0 {
		value = 2.0
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func errorAs(err error, target any) bool {
	switch t := target.(type) {
	case **exec.ExitError:
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false
		}
		*t = exitErr
		return true
	default:
		return false
	}
}

var _ application.SandboxRunner = (*DockerRunner)(nil)
