package synthesizer

import (
    "context"
    "gaia/internal/models"
    "gaia/internal/providers"
    "gaia/internal/state"
)

type Synthesizer struct {
    llm        providers.LLMProvider
    stateStore state.StateStore
}

func NewSynthesizer(llm providers.LLMProvider, stateStore state.StateStore) *Synthesizer {
    return &Synthesizer{
        llm:        llm,
        stateStore: stateStore,
    }
}

// Synthesize es el Cerebro 2: genera respuesta natural a partir de datos crudos
func (s *Synthesizer) Synthesize(ctx context.Context, sessionID, originalMessage, rawData string) (*models.SynthesisResponse, error) {
    // TODO: Implementar lógica completa
    // 1. Cargar estado
    // 2. Construir prompt con datos crudos + pregunta original
    // 3. Llamar al LLM
    // 4. Devolver respuesta natural
    
    return &models.SynthesisResponse{
        Response: "Respuesta mock sintetizada",
    }, nil
}