// internal/brain/planner/planner.go
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const (
	// Zona horaria por defecto (Paraguay)
	DefaultTimezone = "America/Asuncion"

	// Máximo de mensajes de historial que se inyectan al prompt
	MaxHistoryForPrompt = 6 // 3 turnos = 6 mensajes (user+assistant)
)

type Planner struct {
	llm      providers.LLMProvider
	timezone string
}

func NewPlanner(llm providers.LLMProvider) *Planner {
	return &Planner{
		llm:      llm,
		timezone: DefaultTimezone,
	}
}

// SetTimezone permite sobreescribir la zona horaria
func (p *Planner) SetTimezone(tz string) {
	if tz != "" {
		p.timezone = tz
	}
}

// Analyze es el Cerebro 1: extrae intent, function_name y parameters.
// No calcula missing_slots, no decide status, no genera texto al usuario.
func (p *Planner) Analyze(
	ctx context.Context,
	req models.ChatRequest,
) (*models.AnalysisResponse, error) {

	// 1. Calcular la fecha/hora actual (uso interno)
	now := p.currentTime()

	// 2. Armar el prompt
	systemPrompt := p.buildSystemPrompt(req, now)

	// 3. Preparar mensajes: system + el mensaje actual
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Message},
	}

	// 4. Definir tools
	tools := p.getAnalysisTools()

	// 5. Llamar al LLM
	llmResponse, err := p.llm.Chat(ctx, messages, tools)
	if err != nil {
		return nil, fmt.Errorf("error al llamar al LLM: %w", err)
	}

	// 6. Procesar respuesta
	analysis := &models.AnalysisResponse{
		OriginalInput: req.Message,
		Parameters:    map[string]interface{}{},
	}

	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name != "update_intent" {
				continue
			}

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("Planner: error al parsear argumentos: %v", err)
				continue
			}

			analysis.Intent = toString(args["intent"])
			analysis.FunctionName = toString(args["function_name"])
			analysis.Parameters = toMap(args["parameters"])
			analysis.Confidence = toFloat64(args["confidence"])
		}
	} else {
		// Sin tool call: conversación casual
		analysis.Intent = "conversation"
		analysis.FunctionName = ""
		analysis.Parameters = map[string]interface{}{}
		analysis.Confidence = 1.0
	}

	return analysis, nil
}

// currentTime devuelve la hora actual en la zona configurada
func (p *Planner) currentTime() time.Time {
	loc, err := time.LoadLocation(p.timezone)
	if err != nil {
		log.Printf("Planner: no se pudo cargar timezone %q, usando UTC: %v", p.timezone, err)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

func (p *Planner) buildSystemPrompt(
	req models.ChatRequest,
	now time.Time,
) string {

	collectedDataStr := "(none)"
	if len(req.CollectedData) > 0 {
		collectedDataStr = formatMap(req.CollectedData)
	}

	historyStr := p.formatHistory(req.History)

	return fmt.Sprintf(`You are GAIA, an intent analyzer for enterprise systems.

## Current date and time
%s (timezone: %s)

## Your task
Extract ONLY these fields from the user's message:
1. "intent": General intent in English snake_case (e.g. "query_billing")
2. "function_name": Specific function in English snake_case using an approved verb prefix
3. "parameters": Key data extracted from the message (keys in English, values RAW as the user wrote them)
4. "confidence": Your confidence level (0.0 to 1.0)

## Do NOT do any of these
- Do NOT compute missing slots
- Do NOT decide status (ready/waiting/etc)
- Do NOT generate clarification questions
- Do NOT generate natural-language responses to the user
- Do NOT normalize values (dates, numbers, names stay RAW)

## CRITICAL: Extract RAW, do NOT normalize
You are NOT a normalizer. Another module handles that.
- "hoy" → keep as "hoy"
- "150 mil" → keep as "150 mil"
- "juan ramirez" → keep as "juan ramirez"
- Extract values in the same language and format the user used.

## Function naming rules (STRICT)
- Format: snake_case in ENGLISH
- ALWAYS use one of these approved verbs as prefix:
  - get_       (single record / specific query)
  - list_      (multiple records)
  - create_    (insert)
  - update_    (modify)
  - delete_    (remove)
  - calculate_ (aggregations)
  - send_      (actions that send something)
- NEVER invent new verbs. NEVER mix Spanish and English.

## Reference examples
- "cuanto facture hoy" → intent:"query_billing", function_name:"get_daily_billing", parameters:{"date":"hoy"}, confidence:0.95
- "estado de cuenta de juan ramirez" → intent:"query_client_balance", function_name:"get_client_balance", parameters:{"client":"juan ramirez"}, confidence:0.95
- "listame los clientes" → intent:"list_clients", function_name:"list_clients", parameters:{}, confidence:0.95
- "quiero agendar cita" → intent:"create_appointment", function_name:"create_appointment", parameters:{}, confidence:0.95
- "el lunes con el doctor ramirez" → intent:"create_appointment", function_name:"create_appointment", parameters:{"date":"el lunes","doctor":"ramirez"}, confidence:0.95
- "hola" → intent:"conversation", function_name:"", parameters:{}, confidence:1.0
- "gracias" → intent:"conversation", function_name:"", parameters:{}, confidence:1.0

## Confidence rules (BE CONSERVATIVE)
- 0.9-1.0: User was very clear and specific
- 0.7-0.89: Understood but slight ambiguity
- 0.5-0.69: Ambiguous, could be several things
- 0.0-0.49: Cannot understand the request
DO NOT inflate confidence.

## Typo handling
Ignore typos and spelling mistakes. "kiero el etado" = "quiero el estado". This MUST NOT lower confidence.

## Using collected data and history
- If "collected_data" already has some parameters, and the user's new message
  adds more, include BOTH in your "parameters" output.
- If the user replies with a short value (e.g. "el lunes", "12345", "Juan") and
  the previous intent was clear, REUSE that intent and fill in the new value.
- If the user changes topic, DISCARD the previous intent and start fresh.

## Collected data (already known from previous turns)
%s

## Recent conversation (last turns)
%s

## Current message
"%s"

Use the 'update_intent' function to return your extraction.`,
		now.Format("2006-01-02 15:04:05"),
		p.timezone,
		collectedDataStr,
		historyStr,
		req.Message,
	)
}

func (p *Planner) getAnalysisTools() []providers.Tool {
	return []providers.Tool{
		{
			Type: "function",
			Function: providers.FunctionDef{
				Name:        "update_intent",
				Description: "Extract intent, function_name, parameters and confidence from the user's message",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"intent": map[string]interface{}{
							"type":        "string",
							"description": "General intent in English snake_case (e.g. query_billing)",
						},
						"function_name": map[string]interface{}{
							"type":        "string",
							"description": "Function name in English snake_case using an approved verb prefix (get_, list_, create_, update_, delete_, calculate_, send_). Empty for casual conversation.",
						},
						"parameters": map[string]interface{}{
							"type":        "object",
							"description": "Extracted parameters. Keys in English, values RAW as the user wrote them (do NOT normalize).",
						},
						"confidence": map[string]interface{}{
							"type":        "number",
							"description": "Confidence level (0.0 to 1.0)",
						},
					},
					"required": []string{"intent", "function_name", "parameters", "confidence"},
				},
			},
		},
	}
}

// formatHistory convierte el historial a texto para el prompt (máx 3 turnos)
func (p *Planner) formatHistory(history []models.HistoryEntry) string {
	if len(history) == 0 {
		return "(no previous history)"
	}

	// Tomar los últimos N
	start := 0
	if len(history) > MaxHistoryForPrompt {
		start = len(history) - MaxHistoryForPrompt
	}

	var sb strings.Builder
	for _, entry := range history[start:] {
		label := "user"
		content := entry.Content
		if entry.Role == "assistant" {
			label = "assistant"
			content = entry.Summary
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("2006-01-02 15:04"),
			label,
			content,
		))
	}
	return sb.String()
}

// formatMap imprime un map de forma legible
func formatMap(m map[string]interface{}) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}

// Helpers de conversión
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	if i, ok := v.(int); ok {
		return float64(i)
	}
	return 0
}

func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return make(map[string]interface{})
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return make(map[string]interface{})
}