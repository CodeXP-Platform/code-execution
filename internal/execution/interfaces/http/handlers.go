package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg              config.Config
	languageUseCase  *application.LanguageRegistryUseCase
	getExecutionCase *application.GetExecutionUseCase
	readinessUseCase *application.ReadinessUseCase
	clock            application.Clock
	uuidGenerator    application.UUIDGenerator
}

func NewHandler(
	cfg config.Config,
	languageUseCase *application.LanguageRegistryUseCase,
	getExecutionCase *application.GetExecutionUseCase,
	readinessUseCase *application.ReadinessUseCase,
	clock application.Clock,
	uuidGenerator application.UUIDGenerator,
) *Handler {
	return &Handler{
		cfg:              cfg,
		languageUseCase:  languageUseCase,
		getExecutionCase: getExecutionCase,
		readinessUseCase: readinessUseCase,
		clock:            clock,
		uuidGenerator:    uuidGenerator,
	}
}

func (h *Handler) RegisterRoutes(router gin.IRoutes) {
	router.GET("/health/live", h.live)
	router.GET("/health/ready", h.ready)
	router.GET("/languages", h.listLanguages)
	router.GET("/templates/:language", h.getTemplate)
	router.PUT("/templates/:language", h.updateTemplate)
	router.GET("/executions/:executionId", h.getExecution)
}

func (h *Handler) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": h.cfg.AppName,
		"status":  "UP",
	})
}

func (h *Handler) ready(c *gin.Context) {
	status := h.readinessUseCase.Execute(c.Request.Context(), 2*time.Second)
	c.JSON(status.HTTPCode(), gin.H{
		"service":      h.cfg.AppName,
		"status":       status.Status,
		"dependencies": status.Dependencies,
		"errors":       status.Errors,
	})
}

func (h *Handler) listLanguages(c *gin.Context) {
	items, err := h.languageUseCase.ListEnabled(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) getTemplate(c *gin.Context) {
	language := strings.TrimSpace(c.Param("language"))
	template, err := h.languageUseCase.GetTemplate(c.Request.Context(), language)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *Handler) updateTemplate(c *gin.Context) {
	language := strings.TrimSpace(c.Param("language"))

	var input application.UpdateTemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.languageUseCase.UpdateTemplate(c.Request.Context(), language, input, h.clock, h.uuidGenerator)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *Handler) getExecution(c *gin.Context) {
	executionID := strings.TrimSpace(c.Param("executionId"))
	details, err := h.getExecutionCase.Execute(c.Request.Context(), executionID)
	if err != nil {
		h.handleUseCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, details)
}

func (h *Handler) handleUseCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, application.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
