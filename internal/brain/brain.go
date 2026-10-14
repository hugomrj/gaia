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

type Brain struct {
	Planner     *planner.Planner
	Synthesizer *synthesizer.Synthesizer
	Normalizer  *normalizer.Normalizer
}

func NewBrain(llm providers.LLMProvider) *Brain {
	return &Brain{
		Planner:     planner.NewPlanner(llm),
		Synthesizer: synthesizer.NewSynthesizer(llm),
		Normalizer:  normalizer.New(),
	}
}

func (b *Brain) ProcessMessage(ctx context.Context, req models.ChatRequest) (*models.AnalysisResponse, error) {
	return b.Planner.Analyze(ctx, req)
}

func (b *Brain) ProcessResult(ctx context.Context, req models.SynthesisRequest) (*models.SynthesisResponse, error) {
	return b.Synthesizer.Synthesize(ctx, req)
}

// Normalize aplica reglas determinísticas a los parámetros
func (b *Brain) Normalize(params map[string]interface{}) *models.NormalizeResponse {
	normalized, ok, notOk := b.Normalizer.Normalize(params)
	return &models.NormalizeResponse{
		Parameters:    normalized,
		Normalized:    ok,
		NotNormalized: notOk,
	}
}