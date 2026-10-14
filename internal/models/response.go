// internal/models/response.go
package models

// AnalysisResponse es lo que el Planner devuelve al cliente
type AnalysisResponse struct {
	Intent        string                 `json:"intent"`
	FunctionName  string                 `json:"function_name"`
	Parameters    map[string]interface{} `json:"parameters"`
	Confidence    float64                `json:"confidence"`
	OriginalInput string                 `json:"original_input"`
}

// SynthesisResponse es lo que el Synthesizer devuelve al cliente
type SynthesisResponse struct {
	Response     string        `json:"response"`
	HistoryEntry *HistoryEntry `json:"history_entry"` // ← NUEVO
	Mode         string        `json:"mode"`          // ← NUEVO
}


// NormalizeResponse es lo que el Normalizer devuelve
type NormalizeResponse struct {
	Parameters    map[string]interface{} `json:"parameters"`
	Normalized    []string               `json:"normalized"`
	NotNormalized []string               `json:"not_normalized"`
}