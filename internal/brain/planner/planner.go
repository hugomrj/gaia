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

// Analyze es el Cerebro 1: analiza la intención del usuario
func (p *Planner) Analyze(ctx context.Context, sessionID, userMessage string, clientContext map[string]interface{}) (*models.AnalysisResponse, error) {
	
	// 1. Cargar o crear estado
	dialogState, err := p.stateStore.Load(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("error al cargar estado: %w", err)
	}
	if dialogState == nil {
		dialogState = state.NewDialogState(sessionID)
	}

	// 2. Agregar mensaje del usuario al historial
	dialogState.AddMessage("user", userMessage)

	// 3. Construir el prompt del sistema (EN ESPAÑOL para que la IA responda en español)
	systemPrompt := p.buildSystemPrompt(dialogState, clientContext)

	// 4. Preparar mensajes para el LLM
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
	}
	for _, msg := range dialogState.Conversation {
		messages = append(messages, providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 5. Definir herramientas (Function Calling con nombres en español)
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
			if toolCall.Function.Name == "actualizar_intencion" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					log.Printf("Error al parsear argumentos: %v", err)
					continue
				}

				// Mapeamos las claves en español que devuelve la IA a nuestra struct en inglés
				analysis.Intent = toString(args["intencion"])
				analysis.FunctionName = toString(args["nombre_funcion"])
				analysis.Parameters = toMap(args["parametros"])
				analysis.Confidence = toFloat64(args["confianza"])
				analysis.Status = toString(args["estado"])
				analysis.Clarification = toString(args["clarificacion"])
				analysis.MissingSlots = toStringSlice(args["campos_faltantes"])
			}
		}
	} else {
		// Sin tool calls: respuesta conversacional directa
		analysis.Intent = "conversacion"
		analysis.Status = "completado"
		analysis.Confidence = 1.0
		analysis.Clarification = llmResponse.Content
	}

	// 8. Aplicar lógica de fallback basada en confianza
	p.applyConfidenceLogic(analysis)

	// 9. Actualizar estado interno (en inglés)
	dialogState.Intent = analysis.Intent
	dialogState.FunctionName = analysis.FunctionName
	dialogState.Parameters = analysis.Parameters
	dialogState.Status = analysis.Status

	// 10. Agregar respuesta del asistente al historial
	if analysis.Clarification != "" {
		dialogState.AddMessage("assistant", analysis.Clarification)
	} else if analysis.Status == "listo_para_ejecutar" {
		dialogState.AddMessage("assistant", "Perfecto, estoy procesando tu solicitud...")
	}

	// 11. Guardar estado actualizado
	if err := p.stateStore.Save(ctx, dialogState); err != nil {
		return nil, fmt.Errorf("error al guardar estado: %w", err)
	}

	return analysis, nil
}

func (p *Planner) buildSystemPrompt(state *state.DialogState, clientContext map[string]interface{}) string {
	contextInfo := ""
	if clientContext != nil {
		if systemType, ok := clientContext["system_type"].(string); ok {
			contextInfo += fmt.Sprintf("\n- Tipo de sistema: %s", systemType)
		}
		if domain, ok := clientContext["domain"].(string); ok {
			contextInfo += fmt.Sprintf("\n- Dominio: %s", domain)
		}
	}

	// EL PROMPT ESTÁ EN ESPAÑOL PARA QUE LA IA GENERE LOS NOMBRES EN ESPAÑOL
	return fmt.Sprintf(`Eres GAIA, un analizador de intenciones para sistemas empresariales.

## Tu tarea
Analiza el mensaje del usuario y determina:
1. La intención general (en snake_case, ej: "consultar_facturacion")
2. El nombre específico de la función que debe ejecutarse (ej: "obtener_facturacion_diaria")
3. Los parámetros necesarios (fechas, nombres, etc.)
4. Tu nivel de confianza (0.0 a 1.0)
5. Si faltan datos, identifica qué falta (campos_faltantes)
%s

## Reglas para nombre_funcion y parametros
- Debe ser descriptivo y específico (no genérico)
- Formato nombre_funcion: snake_case en ESPAÑOL
- SIEMPRE extrae los datos clave en "parametros"
- Ejemplos:
  - "cuanto facture hoy" → nombre_funcion: "obtener_facturacion_diaria", parametros: {"periodo": "hoy"}
  - "estado de cuenta de juan ramirez" → nombre_funcion: "obtener_estado_cuenta", parametros: {"cliente": "juan ramirez"}

## Reglas de confianza
- 0.9-1.0: El usuario fue claro y específico.
- 0.7-0.89: Se entiende la intención pero faltan datos o hay ambigüedad leve.
- 0.5-0.69: La intención es ambigua, podría ser varias cosas.
- 0.0-0.49: No se entiende la solicitud.

## Reglas de estado
- "listo_para_ejecutar": Todos los parámetros necesarios están presentes y la confianza es alta (≥0.8)
- "esperando_datos": Faltan parámetros obligatorios pero la intención es clara
- "necesita_aclaracion": La confianza es baja (<0.5) o la solicitud es ambigua

## Manejo de errores de escritura
Ignora typos y errores ortográficos. "kiero el etado" = "quiero el estado". Esto NO debe bajar la confianza.

## Estado actual de la conversación
- Intencion anterior: %s
- Parametros conocidos: %+v
- Estado actual: %s

## Historial de conversación
%s

Usa la función 'actualizar_intencion' para devolver tu análisis.`,
		contextInfo,
		state.Intent,
		state.Parameters,
		state.Status,
		p.formatConversation(state.Conversation),
	)
}

func (p *Planner) getAnalysisTools() []providers.Tool {
	return []providers.Tool{
		{
			Type: "function",
			Function: providers.FunctionDef{
				Name:        "actualizar_intencion", // Nombre de la herramienta en español
				Description: "Actualiza la intención, nombre_funcion, parámetros y nivel de confianza del análisis",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"intencion": map[string]interface{}{
							"type":        "string",
							"description": "La intención general del usuario en snake_case",
						},
						"nombre_funcion": map[string]interface{}{
							"type":        "string",
							"description": "El nombre específico de la función que debe ejecutarse (en español, snake_case)",
						},
						"parametros": map[string]interface{}{
							"type":        "object",
							"description": "Los parámetros extraídos del mensaje del usuario",
						},
						"campos_faltantes": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{"type": "string"},
							"description": "Lista de parámetros que faltan para ejecutar la función",
						},
						"confianza": map[string]interface{}{
							"type":        "number",
							"description": "Nivel de confianza del análisis (0.0 a 1.0)",
						},
						"clarificacion": map[string]interface{}{
							"type":        "string",
							"description": "Pregunta de clarificación si la confianza es baja o faltan datos",
						},
						"estado": map[string]interface{}{
							"type": "string",
							"enum": []string{"listo_para_ejecutar", "esperando_datos", "necesita_aclaracion"},
							"description": "Estado del análisis",
						},
					},
					"required": []string{"intencion", "nombre_funcion", "parametros", "campos_faltantes", "confianza", "clarificacion", "estado"},
				},
			},
		},
	}
}

func (p *Planner) applyConfidenceLogic(analysis *models.AnalysisResponse) {
	switch {
	case analysis.Confidence >= HighConfidenceThreshold:
		if len(analysis.MissingSlots) > 0 {
			analysis.Status = "esperando_datos"
			if analysis.Clarification == "" {
				analysis.Clarification = fmt.Sprintf("Para continuar necesito que me proporciones: %s", strings.Join(analysis.MissingSlots, ", "))
			}
		} else {
			analysis.Status = "listo_para_ejecutar"
			analysis.Clarification = ""
		}
	case analysis.Confidence >= MediumConfidenceThreshold:
		analysis.Status = "necesita_aclaracion"
		if analysis.Clarification == "" {
			analysis.Clarification = fmt.Sprintf("¿Quisiste decir que necesitas '%s'? Por favor confirma o dame más detalles.", analysis.Intent)
		}
	default:
		analysis.Status = "necesita_aclaracion"
		analysis.Intent = "desconocido"
		analysis.FunctionName = ""
		analysis.Parameters = map[string]interface{}{}
		analysis.MissingSlots = []string{}
		if analysis.Clarification == "" {
			analysis.Clarification = "No logré entender tu solicitud. ¿Podrías explicarla de otra forma?"
		}
	}
}

func (p *Planner) formatConversation(messages []state.Message) string {
	if len(messages) == 0 {
		return "(sin historial previo)"
	}
	var result strings.Builder
	for _, msg := range messages {
		result.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return result.String()
}

// Helpers de conversión
func toString(v interface{}) string {
	if v == nil { return "" }
	if s, ok := v.(string); ok { return s }
	return fmt.Sprintf("%v", v)
}
func toFloat64(v interface{}) float64 {
	if v == nil { return 0 }
	if f, ok := v.(float64); ok { return f }
	if i, ok := v.(int); ok { return float64(i) }
	return 0
}
func toMap(v interface{}) map[string]interface{} {
	if v == nil { return make(map[string]interface{}) }
	if m, ok := v.(map[string]interface{}); ok { return m }
	return make(map[string]interface{})
}
func toStringSlice(v interface{}) []string {
	if v == nil { return []string{} }
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok { result = append(result, s) }
		}
		return result
	}
	return []string{}
}