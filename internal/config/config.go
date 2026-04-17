package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName     string
	AppHost     string
	AppPort     string
	PostgresURL string
	RabbitMQURL string
	Eureka      EurekaConfig
	Execution   ExecutionConfig
	Messaging   MessagingConfig
}

type EurekaConfig struct {
	Enabled           bool
	BaseURL           string
	Username          string
	Password          string
	ServiceName       string
	InstanceID        string
	IPAddr            string
	Port              int
	HeartbeatInterval time.Duration
}

type ExecutionConfig struct {
	TimeoutMs     int
	MemoryLimitMb int
	CPULimitMs    int
	Sandbox       SandboxConfig
}

type SandboxConfig struct {
	DockerBinary    string
	PythonImage     string
	JavaScriptImage string
	JavaImage       string
	CPPImage        string
}

type MessagingConfig struct {
	RequestedExchange   string
	RequestedQueue      string
	RequestedRoutingKey string
	ExecutionExchange   string
	StartedRoutingKey   string
	CompletedRoutingKey string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	appName := getEnv("APP_NAME", "code-execution")
	appHost := getEnv("APP_HOST", "localhost")
	appPort := getEnv("APP_PORT", "8082")

	appPortNumber, err := strconv.Atoi(appPort)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_PORT value %q: %w", appPort, err)
	}

	eurekaEnabled, err := getBool("EUREKA_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	eurekaPort, err := getInt("EUREKA_PORT", appPortNumber)
	if err != nil {
		return Config{}, err
	}

	eurekaHeartbeatInterval, err := getDuration("EUREKA_HEARTBEAT_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	eurekaServiceName := getEnv("EUREKA_SERVICE_NAME", appName)
	eurekaInstanceID := getEnv("EUREKA_INSTANCE_ID", "")
	if eurekaInstanceID == "" {
		eurekaInstanceID = fmt.Sprintf("%s:%s:%d", strings.ToLower(appName), appHost, eurekaPort)
	}

	timeoutMs, err := getInt("EXECUTION_TIMEOUT_MS", 2000)
	if err != nil {
		return Config{}, err
	}

	memoryLimitMb, err := getInt("EXECUTION_MEMORY_LIMIT_MB", 128)
	if err != nil {
		return Config{}, err
	}

	cpuLimitMs, err := getInt("EXECUTION_CPU_LIMIT_MS", 1000)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:     appName,
		AppHost:     appHost,
		AppPort:     appPort,
		PostgresURL: getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Eureka: EurekaConfig{
			Enabled:           eurekaEnabled,
			BaseURL:           getEnv("EUREKA_BASE_URL", "http://localhost:8761/eureka"),
			Username:          getEnv("EUREKA_USERNAME", ""),
			Password:          getEnv("EUREKA_PASSWORD", ""),
			ServiceName:       strings.ToUpper(eurekaServiceName),
			InstanceID:        eurekaInstanceID,
			IPAddr:            getEnv("EUREKA_IP_ADDR", appHost),
			Port:              eurekaPort,
			HeartbeatInterval: eurekaHeartbeatInterval,
		},
		Execution: ExecutionConfig{
			TimeoutMs:     timeoutMs,
			MemoryLimitMb: memoryLimitMb,
			CPULimitMs:    cpuLimitMs,
			Sandbox: SandboxConfig{
				DockerBinary:    getEnv("SANDBOX_DOCKER_BINARY", "docker"),
				PythonImage:     getEnv("SANDBOX_PYTHON_IMAGE", "python:3.12-alpine"),
				JavaScriptImage: getEnv("SANDBOX_JAVASCRIPT_IMAGE", "node:20-alpine"),
				JavaImage:       getEnv("SANDBOX_JAVA_IMAGE", "eclipse-temurin:21-jdk-alpine"),
				CPPImage:        getEnv("SANDBOX_CPP_IMAGE", "gcc:14"),
			},
		},
		Messaging: MessagingConfig{
			RequestedExchange:   getEnv("REQUESTED_EVENT_EXCHANGE", "challenges.solutions.exchange"),
			RequestedQueue:      getEnv("REQUESTED_EVENT_QUEUE", "codeexecution.solution.execution.requested"),
			RequestedRoutingKey: getEnv("REQUESTED_EVENT_ROUTING_KEY", "challenges.solution.execution.requested"),
			ExecutionExchange:   getEnv("EXECUTION_EVENT_EXCHANGE", "codeexecution.exchange"),
			StartedRoutingKey:   getEnv("STARTED_EVENT_ROUTING_KEY", "codeexecution.solution.execution.started"),
			CompletedRoutingKey: getEnv("COMPLETED_EVENT_ROUTING_KEY", "codeexecution.solution.execution.completed"),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func getBool(key string, fallback bool) (bool, error) {
	raw := getEnv(key, strconv.FormatBool(fallback))
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: %w", key, raw, err)
	}
	return parsed, nil
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := getEnv(key, fallback.String())
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", key, raw, err)
	}
	return parsed, nil
}

func getInt(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", key, raw, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("invalid %s value %q: must be > 0", key, raw)
	}
	return parsed, nil
}

func loadDotEnv() error {
	paths := []string{".env", "../.env", "../../.env"}

	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		err := godotenv.Load(cleanPath)
		if err == nil {
			return nil
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load %s: %w", cleanPath, err)
		}
	}

	return nil
}
