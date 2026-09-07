package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
    return &HealthHandler{}
}

func (h *HealthHandler) Health(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "name":    "GAIA",
        "version": "0.1.0",
        "status":  "running",
    })
}