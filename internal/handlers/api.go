// internal/handlers/api.go
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

func (h *APIHandler) Synthesize(c *gin.Context) {
	var req models.SynthesisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.brain.ProcessResult(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}


// Normalize — Normalizer determinístico: POST /v1/normalize
func (h *APIHandler) Normalize(c *gin.Context) {
	var req models.NormalizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := h.brain.Normalize(req.Parameters)
	c.JSON(http.StatusOK, resp)
}