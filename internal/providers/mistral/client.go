/* internal/providers/mistral/client.go */
package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const mistralAPIURL = "https://api.mistral.ai/v1/chat/completions"

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

type mistralResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string               `json:"role"`
			Content   string               `json:"content"`
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

func NewClient() *Client {
	return &Client{
		apiKey: os.Getenv("MISTRAL_API_KEY"),
		model:  "open-mistral-nemo",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Name() string {
	return "mistral"
}

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

	// Reintentos: solo para errores de red y 429
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff: 1s, 2s
			select {
			case <-ctx.Done():
				return nil, models.ErrTimeout()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		// ⚠️ CRÍTICO: recrear el body en CADA intento
		req, err := http.NewRequestWithContext(ctx, "POST", mistralAPIURL, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, models.ErrInternal(fmt.Sprintf("new request: %v", err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, models.ErrTimeout()
			}
			if ctx.Err() != nil {
				return nil, models.ErrTimeout()
			}
			continue // reintentar
		}

		// 429 → reintentar
		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("429: %s", string(body))
			continue
		}

		// 401/403 → auth, sin reintento
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, models.ErrAuth()
		}

		// Otros 4xx/5xx → upstream, sin reintento (excepto 5xx que podríamos reintentar)
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

		// 2xx → decodificar
		defer resp.Body.Close()
		var mistralResp mistralResponse
		if err := json.NewDecoder(resp.Body).Decode(&mistralResp); err != nil {
			return nil, models.ErrUpstream(fmt.Sprintf("decode response: %v", err))
		}
		if len(mistralResp.Choices) == 0 {
			return nil, models.ErrUpstream("Mistral devolvió 0 choices")
		}

		choice := mistralResp.Choices[0]
		return &providers.LLMResponse{
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}, nil
	}

	// Agotó reintentos
	if lastErr != nil {
		// Heurística: si el último error fue 429, es rate limit
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
