package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gaia/internal/models"
	"gaia/internal/providers"
	"gaia/internal/state"
)

const (
	HighConfidenceThreshold   = 0.8
	MediumConfidenceThreshold = 0.5

	// Estados (valores de API en inglés)
	StatusReady        = "ready_to_execute"
	StatusWaitingData  = "waiting_for_data"
	StatusNeedsClear   = "needs_clarification"
	StatusConversation = "conversation"

	// Historial: solo la última interacción va al LLM
	MaxHistoryMessages = 2

	// Historial guardado en estado (para no crecer infinito)
	MaxStoredMessages = 20
)

type Planner struct {
	llm        providers.LLMProvider
	stateStore state.StateStore
}

func NewPlanner(llm providers.LLMProvider, stateStore state.StateStore) *Planner {
	return &Planner{
		llm:        llm,
		stateStore: stateStore,
	}
}

// Analyze es el Cerebro 1: analiza la intención del usuario.
func (p *Planner) Analyze(
	ctx context.Context,
	sessionID, userMessage string,
	clientContext map[string]interface{},
) (*models.AnalysisResponse, error) {

	// 1. Cargar o crear estado
	dialogState, err := p.stateStore.Load(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("error al cargar estado: %w", err)
	}
	if dialogState == nil {
		dialogState = state.NewDialogState(sessionID)
	}

	// 2. Agregar mensaje del usuario al historial interno
	dialogState.AddMessage("user", userMessage)
	p.trimConversation(dialogState)

	// 3. Construir prompt
	systemPrompt := p.buildSystemPrompt(dialogState, clientContext)

	// 4. Preparar mensajes: system + SOLO la última interacción
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
	}
	for _, msg := range p.lastNMessages(dialogState.Conversation, MaxHistoryMessages) {
		messages = append(messages, providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 5. Tools
	tools := p.getAnalysisTools()

	// 6. Llamar al LLM
	llmResponse, err := p.llm.Chat(ctx, messages, tools)
	if err != nil {
		return nil, fmt.Errorf("error al llamar al LLM: %w", err)
	}

	// 7. Procesar respuesta
	analysis := &models.AnalysisResponse{
		OriginalInput: userMessage,
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
			analysis.Status = toString(args["status"])
			analysis.Clarification = toString(args["clarification"])
			analysis.MissingSlots = toStringSlice(args["missing_slots"])
		}
	} else {
		// Sin tool call: conversación casual
		analysis.Intent = "conversation"
		analysis.Status = StatusConversation
		analysis.Confidence = 1.0
		analysis.Clarification = llmResponse.Content
	}

	// 8. Fallback de confianza (solo actúa en casos extremos)
	p.applyConfidenceLogic(analysis)

	// 9. Actualizar estado interno
	dialogState.Intent = analysis.Intent
	dialogState.FunctionName = analysis.FunctionName
	dialogState.Parameters = analysis.Parameters
	dialogState.Status = analysis.Status

	// 10. Fusionar parámetros en slots (hechos confirmados que sobreviven entre turnos)
	if analysis.Status == StatusReady {
		for k, v := range analysis.Parameters {
			dialogState.Slots[k] = v
		}
	}

	// 11. Guardar respuesta del asistente SOLO si hay algo que decir al usuario
	if analysis.Clarification != "" {
		dialogState.AddMessage("assistant", analysis.Clarification)
		p.trimConversation(dialogState)
	}

	// 12. Guardar estado
	if err := p.stateStore.Save(ctx, dialogState); err != nil {
		return nil, fmt.Errorf("error al guardar estado: %w", err)
	}

	return analysis, nil
}

func (p *Planner) buildSystemPrompt(
	dialogState *state.DialogState,
	clientContext map[string]interface{},
) string {

	contextInfo := ""
	if clientContext != nil {
		if systemType, ok := clientContext["system_type"].(string); ok {
			contextInfo += fmt.Sprintf("\n- System type: %s", systemType)
		}
		if domain, ok := clientContext["domain"].(string); ok {
			contextInfo += fmt.Sprintf("\n- Domain: %s", domain)
		}
	}

	return fmt.Sprintf(`You are GAIA, an intent analyzer for enterprise systems.

## Your task
Analyze the user's message and return a structured analysis with:
1. "intent": General intent in English snake_case (e.g. "query_billing")
2. "function_name": Specific function name in English snake_case (e.g. "get_daily_billing")
3. "parameters": Key data extracted from the message (keys in English, values RAW as the user wrote them)
4. "missing_slots": List of required data the user hasn't provided yet
5. "confidence": Your confidence level (0.0 to 1.0)
6. "status": Analysis status (see rules below)
7. "clarification": Natural-language question in SPANISH to ask the user if data is missing or ambiguous
%s

## CRITICAL: Extract RAW, do NOT normalize
You are NOT a normalizer. Another module handles that.
- DO NOT convert "hoy" into a date. Return "hoy".
- DO NOT convert "150 mil" into 150000. Return "150 mil".
- DO NOT convert "juan ramirez" into "Juan Ramirez". Return it as the user wrote it.
- Extract values in the same language and format the user used.
- Examples of RAW extraction:
  - "cuanto facture hoy" → parameters: {"date": "hoy"}
  - "facturacion de este mes" → parameters: {"period": "este mes"}
  - "saldo de juan ramirez" → parameters: {"client": "juan ramirez"}
  - "las ventas del año pasado" → parameters: {"period": "año pasado"}

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
- NEVER invent new verbs. NEVER mix Spanish and English in the function name.
- Examples:
  - "cuanto facture hoy" → function_name: "get_daily_billing"
  - "estado de cuenta de juan ramirez" → function_name: "get_client_balance"
  - "listame los clientes" → function_name: "list_clients"
  - "crea un cliente nuevo" → function_name: "create_client"

## Status rules
- "ready_to_execute": All required data present AND confidence >= 0.8
- "waiting_for_data": Intent is clear but required parameters are missing
- "needs_clarification": Confidence < 0.5 OR the request is ambiguous

## Confidence rules (BE CONSERVATIVE)
- 0.9-1.0: User was very clear and specific
- 0.7-0.89: Understood but minor data missing or slight ambiguity
- 0.5-0.69: Ambiguous, could be several things
- 0.0-0.49: Cannot understand the request
DO NOT inflate confidence. If you doubt, lower the number.

## Typo handling
Ignore typos and spelling mistakes. "kiero el etado" = "quiero el estado". This MUST NOT lower confidence.

## Short answers in context
If the previous status was "waiting_for_data" and the user replies with a short value
(e.g. "12345", "si", "Juan"):
- DO NOT create a new intent
- Reuse the previous intent and function_name
- Fill the missing parameter with the value the user gave (RAW, without normalizing)
- Example: if "date" was missing and the user says "de hoy", return date:"de hoy" and status:"ready_to_execute"

## User corrections
If the user changes topic (e.g. "no, mejor mostrame los clientes"):
- DISCARD the previous intent and function_name
- Start fresh with the new request
- Do NOT merge the two intents

## Casual conversation
If the message is a greeting ("hola"), farewell ("gracias"), or small talk:
- intent: "conversation"
- function_name: ""
- status: "conversation"
- confidence: 1.0
- clarification: a short friendly reply in SPANISH

## Current conversation state
- Previous intent: %s
- Previous function_name: %s
- Confirmed slots (facts already gathered): %+v
- Previous status: %s

## Recent conversation
%s

Use the 'update_intent' function to return your analysis.`,
		contextInfo,
		dialogState.Intent,
		dialogState.FunctionName,
		dialogState.Slots,
		dialogState.Status,
		p.formatConversation(p.lastNMessages(dialogState.Conversation, MaxHistoryMessages)),
	)
}

func (p *Planner) getAnalysisTools() []providers.Tool {
	return []providers.Tool{
		{
			Type: "function",
			Function: providers.FunctionDef{
				Name:        "update_intent",
				Description: "Update the intent, function_name, parameters and confidence of the analysis",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"intent": map[string]interface{}{
							"type":        "string",
							"description": "General intent in English snake_case (e.g. query_billing)",
						},
						"function_name": map[string]interface{}{
							"type":        "string",
							"description": "Function name in English snake_case using an approved verb prefix (get_, list_, create_, update_, delete_, calculate_, send_)",
						},
						"parameters": map[string]interface{}{
							"type":        "object",
							"description": "Extracted parameters. Keys in English, values RAW as the user wrote them (do NOT normalize)",
						},
						"missing_slots": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "List of required parameters that are still missing",
						},
						"confidence": map[string]interface{}{
							"type":        "number",
							"description": "Confidence level (0.0 to 1.0)",
						},
						"clarification": map[string]interface{}{
							"type":        "string",
							"description": "Natural-language clarification question IN SPANISH to show to the user",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"ready_to_execute", "waiting_for_data", "needs_clarification"},
							"description": "Analysis status",
						},
					},
					"required": []string{
						"intent", "function_name", "parameters",
						"missing_slots", "confidence", "clarification", "status",
					},
				},
			},
		},
	}
}

func (p *Planner) applyConfidenceLogic(analysis *models.AnalysisResponse) {
	// Solo forzamos clarificación si la confianza es MUY baja.
	if analysis.Confidence < MediumConfidenceThreshold && analysis.Status != StatusConversation {
		analysis.Status = StatusNeedsClear
		analysis.Intent = "unknown"
		analysis.FunctionName = ""
		analysis.Parameters = map[string]interface{}{}
		analysis.MissingSlots = []string{}
		if analysis.Clarification == "" {
			analysis.Clarification = "No logré entender tu solicitud. ¿Podrías explicarla de otra forma?"
		}
		return
	}

	// Si el LLM dijo "ready" pero marcó slots faltantes, bajamos a waiting_for_data
	if analysis.Status == StatusReady && len(analysis.MissingSlots) > 0 {
		analysis.Status = StatusWaitingData
	}

	// Si falta clarificación estando en waiting, la generamos
	if analysis.Status == StatusWaitingData && analysis.Clarification == "" {
		analysis.Clarification = fmt.Sprintf(
			"Para continuar necesito que me proporciones: %s",
			strings.Join(analysis.MissingSlots, ", "),
		)
	}
}

// lastNMessages devuelve los últimos N mensajes del historial.
func (p *Planner) lastNMessages(msgs []state.Message, n int) []state.Message {
	if len(msgs) <= n {
		return msgs
	}
	return msgs[len(msgs)-n:]
}

// trimConversation recorta el historial guardado para que no crezca infinito.
func (p *Planner) trimConversation(dialogState *state.DialogState) {
	if len(dialogState.Conversation) > MaxStoredMessages {
		start := len(dialogState.Conversation) - MaxStoredMessages
		dialogState.Conversation = dialogState.Conversation[start:]
	}
}

func (p *Planner) formatConversation(messages []state.Message) string {
	if len(messages) == 0 {
		return "(no previous history)"
	}
	var result strings.Builder
	for _, msg := range messages {
		result.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return result.String()
}

// Helpers
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

func toStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}