package models

import "fmt"

type GAIAError struct {
    Code    string
    Message string
    Retry   bool
}

func (e *GAIAError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewLLMError(message string, retry bool) *GAIAError {
    return &GAIAError{
        Code:    "llm_error",
        Message: message,
        Retry:   retry,
    }
}