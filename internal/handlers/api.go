package handlers

import (
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

// Analyze — Cerebro 1: POST /v1/analyze
func (h *APIHandler) Analyze(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysis, err := h.brain.ProcessMessage(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// Synthesize — Cerebro 2: POST /v1/synthesize
func (h *APIHandler) Synthesize(c *gin.Context) {
	var result models.ExecutionResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	synthesis, err := h.brain.ProcessResult(c.Request.Context(), result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, synthesis)
}