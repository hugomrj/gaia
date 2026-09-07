package handlers

import (
	"net/http"

	"gaia/internal/brain"
	"gaia/internal/models"

	"github.com/gin-gonic/gin"
)

type ManejadorAPI struct {
	cerebro *brain.Cerebro
}

func NewManejadorAPI(cerebro *brain.Cerebro) *ManejadorAPI {
	return &ManejadorAPI{cerebro: cerebro}
}

// Analizar es el endpoint: POST /v1/analyze
func (h *ManejadorAPI) Analizar(c *gin.Context) {
	var req models.SolicitudChat
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analisis, err := h.cerebro.ProcesarMensaje(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analisis)
}

// Sintetizar es el endpoint: POST /v1/synthesize
func (h *ManejadorAPI) Sintetizar(c *gin.Context) {
	var resultado models.ResultadoEjecucion
	if err := c.ShouldBindJSON(&resultado); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sintesis, err := h.cerebro.ProcesarResultado(c.Request.Context(), resultado)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sintesis)
}