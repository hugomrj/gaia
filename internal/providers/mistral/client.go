// internal/providers/mistral/client.go 
package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const mistralAPIURL = "https://api.mistral.ai/v1/chat/completions"

// defaultModel es el modelo que se usa si el cliente NO envía el header X-Model.
const defaultModel = "open-mistral-nemo"

type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

type mistralRequest struct {
	Model       string              `json:"model"`
	Messages    []providers.Message `json:"messages"`
	Tools       []providers.Tool    `json:"tools,omitempty"`
	ToolChoice  string              `json:"tool_choice,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

// flexibleString maneja el campo "content" que puede venir como string
// (modelos viejos) o como array de bloques (modelos nuevos).
type flexibleString string

func (f *flexibleString) UnmarshalJSON(data []byte) error {
	// 1. Intentar parsear como string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexibleString(s)
		return nil
	}

	// 2. Intentar parsear como array de bloques de texto
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &arr); err == nil {
		var sb strings.Builder
		for _, item := range arr {
			if item.Type == "text" {
				sb.WriteString(item.Text)
			}
		}
		*f = flexibleString(sb.String())
		return nil
	}

	return fmt.Errorf("formato de content desconocido: %s", string(data))
}

type mistralResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string               `json:"role"`
			Content   flexibleString       `json:"content"` // <-- CAMBIO CLAVE ACÁ
			ToolCalls []providers.ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewClient construye un cliente Mistral con la API key del tenant (BYOK).
// Si model viene vacío, usa defaultModel.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}

	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Name() string { return "mistral" }

func (c *Client) Chat(ctx context.Context, messages []providers.Message, tools []providers.Tool) (*providers.LLMResponse, error) {
	if c.apiKey == "" {
		return nil, models.ErrAuth()
	}

	reqBody := mistralRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   1024,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = "auto"
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, models.ErrInternal(fmt.Sprintf("marshal request: %v", err))
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, models.ErrTimeout()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", mistralAPIURL, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, models.ErrInternal(fmt.Sprintf("new request: %v", err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil, models.ErrTimeout()
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("429: %s", string(body))
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, models.ErrAuth()
		}
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, models.ErrUpstream(fmt.Sprintf("Mistral %d: %s", resp.StatusCode, truncate(string(body), 200)))
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, models.ErrInvalidRequest(fmt.Sprintf("Mistral %d: %s", resp.StatusCode, truncate(string(body), 200)))
		}

		defer resp.Body.Close()
		var mr mistralResponse
		if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
			return nil, models.ErrUpstream(fmt.Sprintf("decode response: %v", err))
		}
		if len(mr.Choices) == 0 {
			return nil, models.ErrUpstream("Mistral devolvió 0 choices")
		}

		// Convertimos el flexibleString de vuelta a string normal
		return &providers.LLMResponse{
			Content:   string(mr.Choices[0].Message.Content),
			ToolCalls: mr.Choices[0].Message.ToolCalls,
		}, nil
	}

	if lastErr != nil {
		if s := lastErr.Error(); len(s) > 3 && s[:3] == "429" {
			return nil, models.ErrRateLimit()
		}
		return nil, models.ErrUpstream(fmt.Sprintf("reintentos agotados: %v", lastErr))
	}
	return nil, models.ErrUpstream("reintentos agotados")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}