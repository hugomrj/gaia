package mistral

import (
    "bytes"
    "context"
    "encoding/json"
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
            Role      string              `json:"role"`
            Content   string              `json:"content"`
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
        // Cambiamos a un modelo disponible en el tier gratuito
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
        return nil, &models.GAIAError{
            Code:    "missing_api_key",
            Message: "MISTRAL_API_KEY no está configurada",
            Retry:   false,
        }
    }

    // Construir request
    reqBody := mistralRequest{
        Model:       c.model,
        Messages:    messages,
        Temperature: 0.1, // Baja temperatura para respuestas más determinísticas
        MaxTokens:   1024,
    }

    if len(tools) > 0 {
        reqBody.Tools = tools
        reqBody.ToolChoice = "auto" // El LLM decide si usar herramientas
    }

    jsonBody, err := json.Marshal(reqBody)
    if err != nil {
        return nil, &models.GAIAError{
            Code:    "marshal_error",
            Message: fmt.Sprintf("Error al serializar request: %v", err),
            Retry:   false,
        }
    }

    // Crear HTTP request
    req, err := http.NewRequestWithContext(ctx, "POST", mistralAPIURL, bytes.NewBuffer(jsonBody))
    if err != nil {
        return nil, &models.GAIAError{
            Code:    "request_error",
            Message: fmt.Sprintf("Error al crear request: %v", err),
            Retry:   false,
        }
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.apiKey)

    // Ejecutar request con reintentos
    var resp *http.Response
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        resp, err = c.httpClient.Do(req)
        if err != nil {
            if attempt == maxRetries-1 {
                return nil, &models.GAIAError{
                    Code:    "network_error",
                    Message: fmt.Sprintf("Error de red: %v", err),
                    Retry:   true,
                }
            }
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        break
    }
    defer resp.Body.Close()

    // Manejar errores HTTP
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        
        retry := resp.StatusCode == 429 || resp.StatusCode >= 500
        return nil, &models.GAIAError{
            Code:    fmt.Sprintf("http_%d", resp.StatusCode),
            Message: fmt.Sprintf("Mistral API error: %s", string(body)),
            Retry:   retry,
        }
    }

    // Decodificar respuesta
    var mistralResp mistralResponse
    if err := json.NewDecoder(resp.Body).Decode(&mistralResp); err != nil {
        return nil, &models.GAIAError{
            Code:    "decode_error",
            Message: fmt.Sprintf("Error al decodificar respuesta: %v", err),
            Retry:   false,
        }
    }

    if len(mistralResp.Choices) == 0 {
        return nil, &models.GAIAError{
            Code:    "empty_response",
            Message: "Mistral no devolvió ninguna respuesta",
            Retry:   true,
        }
    }

    // Convertir a LLMResponse genérico
    choice := mistralResp.Choices[0]
    return &providers.LLMResponse{
        Content:   choice.Message.Content,
        ToolCalls: choice.Message.ToolCalls,
    }, nil
}