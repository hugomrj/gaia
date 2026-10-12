// internal/brain/synthesizer/synthesizer.go
package synthesizer

import (
	"context"
	"fmt"
	"log"

	"gaia/internal/models"
	"gaia/internal/providers"
	"gaia/internal/state"
)

type Synthesizer struct {
	llm        providers.LLMProvider
	stateStore state.StateStore
}

func NewSynthesizer(llm providers.LLMProvider, stateStore state.StateStore) *Synthesizer {
	return &Synthesizer{
		llm:        llm,
		stateStore: stateStore,
	}
}

// Synthesize es el Cerebro 2: genera respuesta natural a partir de datos crudos
func (s *Synthesizer) Synthesize(
	ctx context.Context,
	sessionID, originalMessage, rawData string,
) (*models.SynthesisResponse, error) {

	// 1. Cargar estado (para saber qué intención/función se ejecutó)
	dialogState, err := s.stateStore.Load(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("error al cargar estado: %w", err)
	}

	// 2. Construir el prompt del sistema
	systemPrompt := s.buildSystemPrompt(dialogState, originalMessage, rawData)

	// 3. Armar los mensajes para el LLM (sin tools, solo texto)
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: originalMessage},
	}

	// 4. Llamar al LLM
	llmResponse, err := s.llm.Chat(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("error al llamar al LLM en síntesis: %w", err)
	}

	if llmResponse.Content == "" {
		return nil, fmt.Errorf("el LLM devolvió una respuesta vacía en síntesis")
	}

	// 5. Guardar la respuesta final en el historial
	if dialogState != nil {
		dialogState.AddMessage("assistant", llmResponse.Content)
		dialogState.Status = "completado"
		if err := s.stateStore.Save(ctx, dialogState); err != nil {
			// No abortamos la respuesta por un fallo al guardar
			log.Printf("Synthesizer: error al guardar estado: %v", err)
		}
	}

	return &models.SynthesisResponse{
		Response: llmResponse.Content,
	}, nil
}

func (s *Synthesizer) buildSystemPrompt(
	dialogState *state.DialogState,
	originalMessage, rawData string,
) string {

	contextInfo := "(sin contexto previo)"
	if dialogState != nil {
		contextInfo = fmt.Sprintf(
			"- Intención detectada: %s\n- Función ejecutada: %s\n- Parámetros usados: %+v",
			dialogState.Intent,
			dialogState.FunctionName,
			dialogState.Parameters,
		)
	}

	return fmt.Sprintf(`Eres GAIA, el asistente conversacional de un sistema empresarial.

## Tu tarea
Recibes DATOS CRUDOS que el sistema del cliente (ERP, CRM, etc.) devolvió tras ejecutar una función, y debes transformarlos en una respuesta natural, amable y profesional para el usuario final.

## Reglas estrictas
- Responde SIEMPRE en español.
- NO inventes datos. Usa únicamente lo que esté en los DATOS CRUDOS.
- Si los datos están vacíos o son ambiguos, dilo con naturalidad (ej: "No encontré registros para ese período").
- Sé conciso: 1 a 3 frases suelen ser suficientes.
- Formatea números y fechas de forma legible:
  - Montos: "150.000 Gs." (con separador de miles)
  - Fechas: "15 de marzo de 2024" (no "2024-03-15")
  - Porcentajes: "25%%"
- NUNCA menciones que eres una IA, ni que "sintetizas", ni que "recibiste datos crudos". Responde como si tú mismo supieras el dato.
- No saludes de nuevo si ya hay historial. Ve directo a la respuesta.
- Si los datos crudos están en formato JSON, interprétalos y preséntalos de forma legible.

## Ejemplos de transformación
DATOS CRUDOS: "150000"
RESPUESTA: "Hoy has facturado un total de 150.000 Gs."

DATOS CRUDOS: {"cliente": "Juan Ramirez", "saldo": 500000}
RESPUESTA: "El saldo actual de Juan Ramirez es de 500.000 Gs."

DATOS CRUDOS: ""
RESPUESTA: "No encontré información para tu consulta. ¿Podrías verificar los datos?"

## Contexto del diálogo
%s

## DATOS CRUDOS del sistema del cliente
%s

## PREGUNTA ORIGINAL del usuario
%s

Genera ahora la respuesta final para el usuario.`,
		contextInfo,
		rawData,
		originalMessage,
	)
}