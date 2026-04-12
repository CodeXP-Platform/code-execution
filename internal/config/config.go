package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName     string
	AppHost     string
	AppPort     string
	PostgresURL string
	RabbitMQURL string
	Eureka      EurekaConfig
}

type EurekaConfig struct {
	Enabled           bool
	BaseURL           string
	Username          string
	Password          string
	ServiceName       string
	InstanceID        string
	IPAddr            string
	HeartbeatInterval time.Duration
}

func Load() (Config, error) {
	appName := getEnv("APP_NAME", "code-execution-api")
	appHost := getEnv("APP_HOST", "localhost")
	appPort := getEnv("APP_PORT", "8080")

	if _, err := strconv.Atoi(appPort); err != nil {
		return Config{}, fmt.Errorf("invalid APP_PORT value %q: %w", appPort, err)
	}

	eurekaEnabled, err := getBool("EUREKA_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	eurekaHeartbeatInterval, err := getDuration("EUREKA_HEARTBEAT_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	eurekaServiceName := getEnv("EUREKA_SERVICE_NAME", appName)
	eurekaInstanceID := getEnv("EUREKA_INSTANCE_ID", fmt.Sprintf("%s:%s:%s", strings.ToLower(appName), appHost, appPort))

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
			HeartbeatInterval: eurekaHeartbeatInterval,
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
