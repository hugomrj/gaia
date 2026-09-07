package models

// ChatRequest es lo que el cliente envía a GAIA
type ChatRequest struct {
    SessionID     string                 `json:"session_id" binding:"required"`
    Message       string                 `json:"message" binding:"required"`
    ClientContext map[string]interface{} `json:"client_context"` // Tipo de sistema, dominio, etc.
}

// ExecutionResult es lo que el cliente envía después de ejecutar la función
type ExecutionResult struct {
    SessionID string `json:"session_id" binding:"required"`
    Data      string `json:"data"` // Datos crudos del ERP
}