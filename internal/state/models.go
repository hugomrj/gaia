package state

import "time"

type Message struct {
    Role      string    `json:"role"` // "user", "assistant", "system"
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

type DialogState struct {
    SessionID    string                 `json:"session_id"`
    Intent       string                 `json:"intent"`
    FunctionName string                 `json:"function_name"`
    Parameters   map[string]interface{} `json:"parameters"`
    Status       string                 `json:"status"`
    Conversation []Message              `json:"conversation"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
}

func NewDialogState(sessionID string) *DialogState {
    now := time.Now()
    return &DialogState{
        SessionID:    sessionID,
        Parameters:   make(map[string]interface{}),
        Conversation: []Message{},
        Status:       "new",
        CreatedAt:    now,
        UpdatedAt:    now,
    }
}

func (d *DialogState) AddMessage(role, content string) {
    d.Conversation = append(d.Conversation, Message{
        Role:      role,
        Content:   content,
        Timestamp: time.Now(),
    })
    d.UpdatedAt = time.Now()
}