/* internal/models/errors.go */
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
	return &GAIAError{Code: "llm_error", Message: message, Retry: retry}
}

func ErrRateLimit() *GAIAError {
	return &GAIAError{Code: "rate_limit", Message: "rate limit del proveedor LLM, reintentá en unos segundos", Retry: true}
}
func ErrTimeout() *GAIAError {
	return &GAIAError{Code: "timeout", Message: "el LLM no respondió a tiempo", Retry: true}
}
func ErrAuth() *GAIAError {
	return &GAIAError{Code: "auth", Message: "credenciales del LLM inválidas o ausentes", Retry: false}
}
func ErrUpstream(msg string) *GAIAError {
	return &GAIAError{Code: "upstream", Message: msg, Retry: true}
}
func ErrInvalidRequest(msg string) *GAIAError {
	return &GAIAError{Code: "invalid_request", Message: msg, Retry: false}
}
func ErrInternal(msg string) *GAIAError {
	return &GAIAError{Code: "internal_error", Message: msg, Retry: false}
}