# Code Execution API (Go + Gin)

Proyecto base para una API REST en Go usando Gin, con integración de:

- PostgreSQL
- RabbitMQ
- Eureka Service Discovery

Toda la configuración se inyecta por variables de entorno.

## Requisitos

- Go 1.22+
- Docker y Docker Compose (opcional para dependencias)

## Variables de entorno

Usa el archivo `.env.example` como referencia.

Variables principales:

- `APP_NAME`
- `APP_HOST`
- `APP_PORT`
- `POSTGRES_URL`
- `RABBITMQ_URL`
- `EUREKA_ENABLED`
- `EUREKA_BASE_URL`
- `EUREKA_USERNAME`
- `EUREKA_PASSWORD`
- `EUREKA_SERVICE_NAME`
- `EUREKA_INSTANCE_ID`
- `EUREKA_IP_ADDR`
- `EUREKA_HEARTBEAT_INTERVAL`

## Levantar dependencias locales

```bash
docker compose up -d
```

Esto inicia servicios con puertos estándar:

- PostgreSQL en `5432`
- RabbitMQ en `5672` (UI en `15672`)
- Eureka en `8761`

## Ejecutar API

```bash
go mod tidy
go run ./cmd/api
```

## Endpoints iniciales

- `GET /health`
- `GET /api/v1/ping`

## Comportamiento con Eureka

- Si `EUREKA_ENABLED=true`, el servicio:
  - se registra en Eureka al iniciar,
  - envía heartbeats periódicos,
  - se desregistra al apagarse.
