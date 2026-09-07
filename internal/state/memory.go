package state

import (
    "context"
    "sync"
)

// MemoryStore es una implementación en memoria (para desarrollo)
type MemoryStore struct {
    mu     sync.RWMutex
    states map[string]*DialogState
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        states: make(map[string]*DialogState),
    }
}

func (m *MemoryStore) Load(ctx context.Context, sessionID string) (*DialogState, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    state, exists := m.states[sessionID]
    if !exists {
        return nil, nil
    }
    return state, nil
}

func (m *MemoryStore) Save(ctx context.Context, state *DialogState) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.states[state.SessionID] = state
    return nil
}

func (m *MemoryStore) Delete(ctx context.Context, sessionID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    delete(m.states, sessionID)
    return nil
}

// GetActiveSessions devuelve el número de sesiones activas (útil para el admin)
func (m *MemoryStore) GetActiveSessions() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return len(m.states)
}

// GetAllStates devuelve todos los estados (para debugging)
func (m *MemoryStore) GetAllStates() map[string]*DialogState {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    copy := make(map[string]*DialogState)
    for k, v := range m.states {
        copy[k] = v
    }
    return copy
}