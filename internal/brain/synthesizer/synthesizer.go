// internal/brain/synthesizer/synthesizer.go
package synthesizer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const (
	DefaultTimezone     = "America/Asuncion"
	MaxHistoryForPrompt = 6 // 3 turnos = 6 mensajes
)

type Synthesizer struct {
	llm      providers.LLMProvider
	timezone string
}

func NewSynthesizer(llm providers.LLMProvider) *Synthesizer {
	return &Synthesizer{
		llm:      llm,
		timezone: DefaultTimezone,
	}
}

func (s *Synthesizer) SetTimezone(tz string) {
	if tz != "" {
		s.timezone = tz
	}
}

// Synthesize redacta la respuesta final al usuario.
func (s *Synthesizer) Synthesize(
	ctx context.Context,
	req models.SynthesisRequest,
) (*models.SynthesisResponse, error) {

	mode := s.detectMode(req)
	now := s.currentTime()
	systemPrompt := s.buildSystemPrompt(req, mode, now)

	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
	}

	llmResponse, err := s.llm.Chat(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("error al llamar al LLM en síntesis: %w", err)
	}

	if llmResponse.Content == "" {
		return nil, fmt.Errorf("el LLM devolvió una respuesta vacía en síntesis")
	}

	return &models.SynthesisResponse{
		Response:     strings.TrimSpace(llmResponse.Content),
		HistoryEntry: buildHistoryEntry(mode, req, now),
		Mode:         mode,
	}, nil
}

func (s *Synthesizer) detectMode(req models.SynthesisRequest) string {
	if req.Intent == "conversation" {
		return "conversation"
	}
	if req.RawData != "" {
		if looksLikeError(req.RawData) {
			return "error"
		}
		return "respond"
	}
	return "clarify"
}

func looksLikeError(s string) bool {
	lower := strings.ToLower(s)
	keywords := []string{
		"error", "timeout", "fallo", "failed", "exception",
		"no encontrado", "not found", "unavailable",
	}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func (s *Synthesizer) currentTime() time.Time {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		log.Printf("Synthesizer: timezone inválida %q, usando UTC", s.timezone)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

// buildHistoryEntry genera el resumen del turno para el historial del cliente.
// Se hace en Go, sin LLM, para ser determinístico y barato.
func buildHistoryEntry(mode string, req models.SynthesisRequest, now time.Time) *models.HistoryEntry {
	var summary string
	switch mode {
	case "conversation":
		summary = "respondí charla casual"
	case "clarify":
		missing := strings.Join(req.MissingSlots, ", ")
		summary = fmt.Sprintf("pedí %s", missing)
	case "respond":
		summary = fmt.Sprintf("respondí sobre %s", req.Intent)
	case "error":
		summary = fmt.Sprintf("error al procesar %s", req.Intent)
	default:
		summary = "respondí"
	}
	return &models.HistoryEntry{
		Role:      "assistant",
		Summary:   summary,
		Timestamp: now.Truncate(time.Second),
	}
}

func (s *Synthesizer) buildSystemPrompt(
	req models.SynthesisRequest,
	mode string,
	now time.Time,
) string {

	historyStr := s.formatHistory(req.History)
	paramsStr := formatMap(req.Parameters)

	base := fmt.Sprintf(`Eres GAIA, el asistente de un sistema empresarial.
Hablas SIEMPRE en español, con tono cercano pero profesional.
NUNCA menciones que eres una IA, ni que "sintetizas", ni que "recibiste datos".

## Fecha y hora actual
%s (zona: %s)

## Contexto del diálogo
- Intent: %s
- Función: %s
- Parámetros ya recopilados: %s

## Mensaje original del usuario
"%s"

## Historial reciente
%s
`,
		now.Format("2006-01-02 15:04"),
		s.timezone,
		req.Intent,
		req.FunctionName,
		paramsStr,
		req.OriginalMessage,
		historyStr,
	)

	switch mode {

	case "conversation":
		return base + `
## Tu tarea (CHARLA CASUAL)
El usuario hizo un saludo, agradecimiento o charla sin pedir una acción concreta.
Responde breve (1-2 frases), amable y directo.
NO inventes que tienes que ejecutar algo.
Si ya hay historial con mensajes previos, NO saludes de nuevo.

Genera la respuesta ahora.`

	case "clarify":
		return base + fmt.Sprintf(`
## Tu tarea (PEDIR DATOS FALTANTES)
El usuario quiere: %s.
Ya dio: %s.
Falta que proporcione: %s.

Genera UNA sola pregunta natural en español pidiendo SOLO los datos que faltan.
- No pidas lo que ya tiene.
- No inventes datos.
- Sé conciso (1-2 frases).
- Tono amable, sin sonar robótico.
- NO hagas preguntas adicionales ni agregues opciones.

Genera la pregunta ahora.`,
			req.Intent,
			paramsStr,
			strings.Join(req.MissingSlots, ", "),
		)

	case "respond":
		return base + fmt.Sprintf(`
## Tu tarea (RESPONDER CON DATOS DEL SISTEMA)
El usuario pidió: %s.
El sistema ejecutó la función y devolvió estos DATOS CRUDOS:

%s

Genera una respuesta natural en español presentando el resultado.
Reglas:
- Usa SOLO los datos proporcionados. NO inventes.
- Formatea números y fechas (ej: "150.000 Gs.").
- Sé conciso (1-3 frases).
- No menciones "el sistema devolvió" ni cosas técnicas.
- NO hagas preguntas adicionales salvo un cierre simple tipo "¿Necesitás algo más?".

Genera la respuesta ahora.`,
			req.Intent,
			req.RawData,
		)

	case "error":
		return base + `
## Tu tarea (ERROR DEL SISTEMA)
El sistema del cliente falló al intentar ejecutar la operación.
El usuario NO debe enterarse del error técnico.

Reglas:
- Discúlpate breve y claramente (1-2 frases).
- NO pidas datos nuevos.
- NO hagas preguntas sobre la operación.
- NO inventes una nueva pregunta ni ofrezcas opciones.
- Sugiere intentar de nuevo más tarde.

Ejemplo: "No pude completar la operación en este momento. ¿Podés intentar de nuevo en unos minutos?"

Genera la respuesta ahora.`
	}

	return base
}

func (s *Synthesizer) formatHistory(history []models.HistoryEntry) string {
	if len(history) == 0 {
		return "(sin historial)"
	}
	start := 0
	if len(history) > MaxHistoryForPrompt {
		start = len(history) - MaxHistoryForPrompt
	}
	var sb strings.Builder
	for _, entry := range history[start:] {
		label := "Usuario"
		content := entry.Content
		if entry.Role == "assistant" {
			label = "Asistente"
			content = entry.Summary
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("15:04"), label, content))
	}
	return sb.String()
}

func formatMap(m map[string]interface{}) string {
	if len(m) == 0 {
		return "(ninguno)"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}