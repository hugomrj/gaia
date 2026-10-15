// internal/models/request.go
package models

import "time"

// ChatRequest es lo que el cliente envía a GAIA para analizar
type ChatRequest struct {
	SessionID    string                 `json:"session_id,omitempty"`
	Message      string                 `json:"message" binding:"required"`
	FunctionName string                 `json:"function_name,omitempty"` // definido por el cliente
	DataSlots    map[string]interface{} `json:"data_slots,omitempty"`    // null = falta
	History      []HistoryEntry         `json:"history,omitempty"`
}


// HistoryEntry es un mensaje resumido del historial (lo maneja el cliente)
type HistoryEntry struct {
	Role      string    `json:"role"`              // "user" | "assistant"
	Content   string    `json:"content,omitempty"` // para mensajes del user
	Summary   string    `json:"summary,omitempty"` // para respuestas del assistant
	Timestamp time.Time `json:"timestamp"`
}

// SynthesisRequest es lo que el cliente envía al Synthesizer
type SynthesisRequest struct {
	SessionID         string                 `json:"session_id,omitempty"`
	OriginalMessage   string                 `json:"original_message" binding:"required"`
	Intent            string                 `json:"intent" binding:"required"`
	FunctionName      string                 `json:"function_name,omitempty"`
	IsConversational  bool                   `json:"is_conversational,omitempty"` // <-- ¡AGRÉGALO!
	Parameters        map[string]interface{} `json:"parameters,omitempty"`
	DataSlots         map[string]interface{} `json:"data_slots,omitempty"`
	RawData           string                 `json:"raw_data,omitempty"`
	ErrorCode         string                 `json:"error_code,omitempty"`
	Confidence        float64                `json:"confidence,omitempty"`
	History           []HistoryEntry         `json:"history,omitempty"`
}

// NormalizeRequest es lo que el cliente envía al Normalizer
type NormalizeRequest struct {
	Parameters map[string]interface{} `json:"parameters" binding:"required"`
}