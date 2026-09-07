package brain

import (
	"context"
	"fmt"

	"gaia/internal/brain/planner"
	"gaia/internal/brain/synthesizer"
	"gaia/internal/models"
	"gaia/internal/providers"
	"gaia/internal/state"
)

type Brain struct {
	Planner     *planner.Planner
	Synthesizer *synthesizer.Synthesizer
	StateStore  state.StateStore
}

func NewBrain(llm providers.LLMProvider, stateStore state.StateStore) *Brain {
	return &Brain{
		Planner:     planner.NewPlanner(llm, stateStore),
		Synthesizer: synthesizer.NewSynthesizer(llm, stateStore),
		StateStore:  stateStore,
	}
}

func (b *Brain) ProcessMessage(ctx context.Context, req models.ChatRequest) (*models.AnalysisResponse, error) {
	return b.Planner.Analyze(ctx, req.SessionID, req.Message, req.ClientContext)
}

func (b *Brain) ProcessResult(ctx context.Context, result models.ExecutionResult) (*models.SynthesisResponse, error) {
	state, err := b.StateStore.Load(ctx, result.SessionID)
	if err != nil {
		return nil, err
	}
	
	var originalMessage string
	for i := len(state.Conversation) - 1; i >= 0; i-- {
		if state.Conversation[i].Role == "user" {
			originalMessage = state.Conversation[i].Content
			break
		}
	}
	
	if originalMessage == "" {
		return nil, fmt.Errorf("no se encontró el mensaje original en el historial")
	}
	
	return b.Synthesizer.Synthesize(ctx, result.SessionID, originalMessage, result.Data)
}