package providers

import (
    "context"
)

// Message representa un mensaje en la conversación
type Message struct {
    Role    string `json:"role"`    // "system", "user", "assistant", "tool"
    Content string `json:"content"`
}

// Tool define una herramienta que el LLM puede usar (Function Calling)
type Tool struct {
    Type     string       `json:"type"` // "function"
    Function FunctionDef  `json:"function"`
}

type FunctionDef struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall representa una llamada a función que el LLM quiere hacer
type ToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"` // "function"
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"` // JSON string
    } `json:"function"`
}

// LLMResponse es la respuesta genérica de cualquier proveedor
type LLMResponse struct {
    Content   string     `json:"content"`
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// LLMProvider define la interfaz que todos los proveedores deben implementar
type LLMProvider interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)
    Name() string
}