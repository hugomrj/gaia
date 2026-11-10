// internal/handlers/api.go
package handlers

import (
	"errors"
	"log"
	"net/http"

	"gaia/internal/brain"
	"gaia/internal/models"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	brain *brain.Brain
}

func NewAPIHandler(b *brain.Brain) *APIHandler {
	return &APIHandler{brain: b}
}

func (h *APIHandler) Analyze(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}
	analysis, err := h.brain.ProcessMessage(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, analysis)
}

func (h *APIHandler) Synthesize(c *gin.Context) {
	var req models.SynthesisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}
	resp, err := h.brain.ProcessResult(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *APIHandler) Normalize(c *gin.Context) {
	var req models.NormalizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}
	resp := h.brain.Normalize(req.Parameters)
	c.JSON(http.StatusOK, resp)
}

// writeError mapea errores de negocio a HTTP status + JSON estructurado.
func writeError(c *gin.Context, err error) {
	var gErr *models.GAIAError
	if !errors.As(err, &gErr) {
		log.Printf("[API] error no estructurado: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "error interno",
			"retry":   false,
		})
		return
	}

	status := http.StatusInternalServerError
	switch gErr.Code {
	case "rate_limit":
		status = http.StatusTooManyRequests // 429
	case "timeout":
		status = http.StatusGatewayTimeout // 504
	case "auth":
		status = http.StatusBadGateway // 502
	case "upstream":
		status = http.StatusServiceUnavailable // 503
	case "invalid_request":
		status = http.StatusBadRequest // 400
	case "internal_error":
		status = http.StatusInternalServerError // 500
	}

	log.Printf("[API] %s: %s (retry=%v)", gErr.Code, gErr.Message, gErr.Retry)

	c.JSON(status, gin.H{
		"error":   gErr.Code,
		"message": gErr.Message,
		"retry":   gErr.Retry,
	})
}


