package server

import (
	"context"
	"net/http"
	"time"

	"code-execution/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func NewRouter(cfg config.Config, pool *pgxpool.Pool, rabbitConn *amqp.Connection) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())

	router.GET("/health", func(c *gin.Context) {
		healthCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		dbErr := pool.Ping(healthCtx)
		rabbitUp := rabbitConn != nil && !rabbitConn.IsClosed()
		eurekaStatus := "disabled"
		if cfg.Eureka.Enabled {
			eurekaStatus = "enabled"
		}

		statusCode := http.StatusOK
		status := "UP"
		if dbErr != nil || !rabbitUp {
			statusCode = http.StatusServiceUnavailable
			status = "DOWN"
		}

		response := gin.H{
			"service": cfg.AppName,
			"status":  status,
			"dependencies": gin.H{
				"postgres": dependencyStatus(dbErr == nil),
				"rabbitmq": dependencyStatus(rabbitUp),
				"eureka":   eurekaStatus,
			},
		}

		if dbErr != nil {
			response["postgres_error"] = dbErr.Error()
		}

		c.JSON(statusCode, response)
	})

	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"service": cfg.AppName,
		})
	})

	return router
}

func dependencyStatus(ok bool) string {
	if ok {
		return "UP"
	}
	return "DOWN"
}
