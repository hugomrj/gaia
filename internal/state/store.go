package state

import "context"

// StateStore define cómo se guarda y carga el estado de las sesiones
type StateStore interface {
    Load(ctx context.Context, sessionID string) (*DialogState, error)
    Save(ctx context.Context, state *DialogState) error
    Delete(ctx context.Context, sessionID string) error
}