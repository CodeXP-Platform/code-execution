# Code Execution API (Go + Gin)

Microservicio de ejecucion de codigo orientado a eventos. Consume solicitudes desde RabbitMQ, ejecuta soluciones en sandbox Docker aislado y publica resultados de ejecucion.

## Requisitos

- Go 1.23+
- Docker y Docker Compose

## Variables de entorno

### Base

- `APP_NAME`
- `APP_HOST`
- `APP_PORT`
- `POSTGRES_URL`
- `RABBITMQ_URL`

### Eureka

- `EUREKA_ENABLED`
- `EUREKA_BASE_URL`
- `EUREKA_USERNAME`
- `EUREKA_PASSWORD`
- `EUREKA_SERVICE_NAME`
- `EUREKA_PORT`
- `EUREKA_INSTANCE_ID`
- `EUREKA_IP_ADDR`
- `EUREKA_HEARTBEAT_INTERVAL`

### Execution limits

- `EXECUTION_TIMEOUT_MS`
- `EXECUTION_MEMORY_LIMIT_MB`
- `EXECUTION_CPU_LIMIT_MS`

### Sandbox images

- `SANDBOX_DOCKER_BINARY`
- `SANDBOX_PYTHON_IMAGE`
- `SANDBOX_JAVASCRIPT_IMAGE`
- `SANDBOX_JAVA_IMAGE`
- `SANDBOX_CPP_IMAGE`

### Messaging

- `REQUESTED_EVENT_EXCHANGE`
- `REQUESTED_EVENT_QUEUE`
- `REQUESTED_EVENT_ROUTING_KEY`
- `EXECUTION_EVENT_EXCHANGE`
- `STARTED_EVENT_ROUTING_KEY`
- `COMPLETED_EVENT_ROUTING_KEY`

Si `EUREKA_PORT` esta vacio, se usa automaticamente el valor de `APP_PORT`.

## Levantar dependencias locales

```bash
docker compose up -d
```

Servicios por defecto:

- PostgreSQL en `5432`
- RabbitMQ en `5672` (UI en `15672`)
- Eureka en `8761`

## Ejecutar API

```bash
go mod tidy
go run ./cmd/api
```

## Endpoints

- `GET /health`
- `GET /api/v1/ping`
- `GET /api/v1/code-execution/health/live`
- `GET /api/v1/code-execution/health/ready`
- `GET /api/v1/code-execution/languages`
- `GET /api/v1/code-execution/templates/{language}`
- `PUT /api/v1/code-execution/templates/{language}`
- `GET /api/v1/code-execution/executions/{executionId}`

## Eventos RabbitMQ

- Consume `challenges.solution.execution.requested`
- Publica `codeexecution.solution.execution.started`
- Publica `codeexecution.solution.execution.completed`

## Patrones aplicados

- Strategy + Factory Method para resolver comportamiento por lenguaje.
- Clean Architecture con capas `domain`, `application`, `infrastructure`, `interfaces`.
- Orquestacion de ejecucion en un caso de uso unico con puertos de entrada/salida.
