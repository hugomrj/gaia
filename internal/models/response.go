package models

type AnalysisResponse struct {
	Intent         string                 `json:"intencion"`
	FunctionName   string                 `json:"nombre_funcion"`
	Parameters     map[string]interface{} `json:"parametros"`
	MissingSlots   []string               `json:"campos_faltantes,omitempty"`
	Status         string                 `json:"estado"` 
	Confidence     float64                `json:"confianza"`
	Clarification  string                 `json:"clarificacion,omitempty"`
	OriginalInput  string                 `json:"entrada_original"`
}

type SynthesisResponse struct {
	Response string `json:"respuesta"`
}