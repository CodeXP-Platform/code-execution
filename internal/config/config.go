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

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	appName := getEnv("APP_NAME", "code-execution")
	appHost := getEnv("APP_HOST", "localhost")
	appPort := getEnv("APP_PORT", "8082")

	fmt.Println("APP_NAME:", appName)
	fmt.Println("APP_HOST:", appHost)
	fmt.Println("APP_PORT:", appPort)

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

	postgresURL := getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://rabbitmq:rabbitmq@localhost:5672/")

	fmt.Println("POSTGRES_URL:", postgresURL)
	fmt.Println("RABBITMQ_URL:", rabbitURL)

	fmt.Println("EUREKA_ENABLED:", eurekaEnabled)
	fmt.Println("EUREKA_BASE_URL:", getEnv("EUREKA_BASE_URL", "http://localhost:8761/eureka"))
	fmt.Println("EUREKA_SERVICE_NAME:", strings.ToUpper(eurekaServiceName))
	fmt.Println("EUREKA_INSTANCE_ID:", eurekaInstanceID)
	fmt.Println("EUREKA_PORT:", eurekaPort)
	fmt.Println("EUREKA_IP_ADDR:", getEnv("EUREKA_IP_ADDR", appHost))
	fmt.Println("EUREKA_HEARTBEAT_INTERVAL:", eurekaHeartbeatInterval)

	return Config{
		AppName:     appName,
		AppHost:     appHost,
		AppPort:     appPort,
		PostgresURL: postgresURL,
		RabbitMQURL: rabbitURL,
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
