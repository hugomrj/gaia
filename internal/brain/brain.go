// internal/brain/brain.go
package brain

import (
	"context"

	"gaia/internal/brain/planner"
	"gaia/internal/brain/synthesizer"
	"gaia/internal/models"
	"gaia/internal/normalizer"
	"gaia/internal/providers"
)

// Brain es una cáscara sin estado: el LLM se inyecta en cada llamada.
type Brain struct {
	Planner     *planner.Planner
	Synthesizer *synthesizer.Synthesizer
	Normalizer  *normalizer.Normalizer
}

func NewBrain() *Brain {
	return &Brain{
		Planner:     planner.NewPlanner(),
		Synthesizer: synthesizer.NewSynthesizer(),
		Normalizer:  normalizer.New(),
	}
}

func (b *Brain) ProcessMessage(ctx context.Context, req models.ChatRequest, llm providers.LLMProvider) (*models.AnalysisResponse, error) {
	return b.Planner.Analyze(ctx, req, llm)
}

func (b *Brain) ProcessResult(ctx context.Context, req models.SynthesisRequest, llm providers.LLMProvider) (*models.SynthesisResponse, error) {
	return b.Synthesizer.Synthesize(ctx, req, llm)
}

// Normalize es 100% determinístico: no usa LLM.
func (b *Brain) Normalize(params map[string]interface{}) *models.NormalizeResponse {
	normalized, ok, notOk := b.Normalizer.Normalize(params)
	return &models.NormalizeResponse{
		Parameters:    normalized,
		Normalized:    ok,
		NotNormalized: notOk,
	}
}