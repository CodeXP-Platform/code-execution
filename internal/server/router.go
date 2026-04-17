package server

import (
	"net/http"

	"code-execution/internal/config"
	executionhttp "code-execution/internal/execution/interfaces/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config, executionHandler *executionhttp.Handler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": cfg.AppName,
			"status":  "UP",
		})
	})

	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"service": cfg.AppName,
		})
	})

	codeExecutionV1 := router.Group("/api/v1/code-execution")
	executionHandler.RegisterRoutes(codeExecutionV1)

	return router
}
