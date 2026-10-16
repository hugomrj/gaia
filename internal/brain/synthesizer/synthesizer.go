package synthesizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const (
	DefaultTimezone        = "America/Asuncion"
	MaxHistoryForPrompt    = 6
	LowConfidenceThreshold = 0.7
)

// Personality define el tono del asistente.
// Cambiá SOLO este bloque para ajustar cómo responde GAIA.
const Personality = `Eres GAIA, el asistente de un sistema empresarial.
Hablás SIEMPRE en español, con tono cercano pero profesional.
Sos directo: respondés sin vueltas, sin adornos innecesarios.
NUNCA mencionás que sos una IA, ni que "sintetizás", ni que "recibís datos".
NUNCA usás emojis.
No hacés preguntas existenciales ni filosóficas.
Tuteás al usuario (vos, tenés, podés).`

// Regex
var (
	reMultiDot = regexp.MustCompile(`\.{2,}$`)
	reNumber   = regexp.MustCompile(`\d{4,}`)
	reNonDigit = regexp.MustCompile(`\D`)
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

/* ============================================================
   SYNTHESIZE
============================================================ */
func (s *Synthesizer) Synthesize(
	ctx context.Context,
	req models.SynthesisRequest,
) (*models.SynthesisResponse, error) {

	now := s.currentTime()

	// 1. Bypass determinístico: fuera de alcance
	if req.FunctionName == "obtener_fuera_alcance" {
		return &models.SynthesisResponse{
			Response:     "No puedo ayudarte con ese tema. Mi función es asistirte con facturación, clientes, stock o citas dentro del sistema.",
			HistoryEntry: buildHistoryEntry("conversation", req, now),
			Mode:         "conversation",
			DataSlots:    req.DataSlots,
			RawData:      req.RawData,
		}, nil
	}

	// 2. Bypass determinístico: función no encontrada
	if req.ErrorCode == "function_not_found" {
		return &models.SynthesisResponse{
			Response:     "No puedo ayudarte con esa solicitud específica. Puedo asistirte con facturación, clientes, stock o citas.",
			HistoryEntry: buildHistoryEntry("error", req, now),
			Mode:         "error",
			DataSlots:    req.DataSlots,
			RawData:      req.RawData,
		}, nil
	}

	// 3. Determinar modo
	mode := s.detectMode(req)

	// 4. Preparar payload
	payload := req.RawData
	if mode == "respond" && req.RawData != "" {
		payload = extractPayload(req.RawData)
	}

	// 5. Construir prompt
	systemPrompt := s.buildSystemPrompt(req, mode, now, payload)

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

	content := strings.TrimSpace(llmResponse.Content)
	content = reMultiDot.ReplaceAllString(content, ".")

	// 6. Guard de números (solo loguea, no rompe)
	if mode == "respond" && payload != "" {
		guardNumbers(content, payload)
	}

	return &models.SynthesisResponse{
		Response:     content,
		HistoryEntry: buildHistoryEntry(mode, req, now),
		Mode:         mode,
		DataSlots:    req.DataSlots,
		RawData:      req.RawData,
	}, nil
}

/* ============================================================
   DETECT MODE
============================================================ */
func (s *Synthesizer) detectMode(req models.SynthesisRequest) string {
	if req.IsConversational {
		return "conversation"
	}
	if req.ErrorCode != "" {
		return "error"
	}
	if req.RawData == "" {
		for _, v := range req.DataSlots {
			if v == nil {
				return "clarify"
			}
		}
		if req.Confidence > 0 && req.Confidence < LowConfidenceThreshold {
			return "clarify"
		}
		return "conversation"
	}
	return "respond"
}

/* ============================================================
   EXTRACT PAYLOAD
============================================================ */
// extractPayload convierte el envelope JSON a texto legible para el LLM.
// Si no es envelope, devuelve el raw tal cual.
func extractPayload(raw string) string {
	var env struct {
		Type  string          `json:"type"`
		Label string          `json:"label"`
		Data  json.RawMessage `json:"data"`
		Meta  json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil || env.Type == "" {
		return raw
	}

	var sb strings.Builder
	sb.WriteString("tipo: " + env.Type + "\n")

	if env.Label != "" {
		sb.WriteString("descripción: " + env.Label + "\n")
	}

	if len(env.Data) > 0 && string(env.Data) != "null" {
		data := strings.TrimSpace(string(env.Data))
		if strings.HasPrefix(data, "\"") && strings.HasSuffix(data, "\"") {
			data = data[1 : len(data)-1]
		}
		sb.WriteString("valor: " + data + "\n")
	}

	if len(env.Meta) > 0 && string(env.Meta) != "null" {
		sb.WriteString("meta: " + string(env.Meta) + "\n")
	}

	return strings.TrimSpace(sb.String())
}

/* ============================================================
   GUARD NUMBERS
============================================================ */
// guardNumbers verifica que el número principal del raw_data esté presente.
// Solo loguea si falla. NO reemplaza la respuesta del LLM.
func guardNumbers(content, rawData string) {
	matches := reNumber.FindAllString(rawData, -1)
	if len(matches) == 0 {
		return
	}

	// Elegir el número más largo como principal
	target := matches[0]
	for _, m := range matches {
		if len(m) > len(target) {
			target = m
		}
	}

	respDigits := reNonDigit.ReplaceAllString(content, "")

	if !strings.Contains(respDigits, target) {
		log.Printf("Synthesizer: ALERTA - Número %q no está en la respuesta. Content: %q", target, content)
	}
}

/* ============================================================
   BUILD HISTORY ENTRY
============================================================ */
func buildHistoryEntry(mode string, req models.SynthesisRequest, now time.Time) *models.HistoryEntry {
	var summary string
	switch mode {
	case "conversation":
		summary = fmt.Sprintf("%s()", req.FunctionName)
	case "clarify":
		var missing []string
		for k, v := range req.DataSlots {
			if v == nil {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			summary = fmt.Sprintf("pedí %s en %s(%s)",
				strings.Join(missing, ", "),
				req.FunctionName,
				formatMap(req.Parameters),
			)
		} else {
			summary = "pedí aclaración"
		}
	case "respond":
		summary = fmt.Sprintf("%s(%s)", req.FunctionName, formatMap(req.Parameters))
	case "error":
		summary = fmt.Sprintf("error %s en %s", req.ErrorCode, req.FunctionName)
	default:
		summary = "respondí"
	}
	return &models.HistoryEntry{
		Role:      "assistant",
		Summary:   summary,
		Timestamp: now.Truncate(time.Second),
	}
}

/* ============================================================
   BUILD SYSTEM PROMPT
============================================================ */
func (s *Synthesizer) buildSystemPrompt(
	req models.SynthesisRequest,
	mode string,
	now time.Time,
	payload string,
) string {

	historyStr := s.formatHistory(req.History)
	paramsStr := formatMap(req.Parameters)

	base := fmt.Sprintf(`%s

## Fecha y hora actual
%s (zona: %s)

## Contexto del diálogo
- Intención: %s
- Función: %s
- Parámetros: %s

## Mensaje original del usuario
"%s"

## Historial reciente
%s
`,
		Personality,
		now.Format("2006-01-02 15:04"),
		s.timezone,
		req.Intent,
		req.FunctionName,
		paramsStr,
		req.OriginalMessage,
		historyStr,
	)

	switch mode {

	/* ---------- CONVERSACIÓN ---------- */
	case "conversation":
		return base + `
## Tu tarea (CHARLA CASUAL)
El usuario hizo un saludo, una pregunta general o charla sin acción concreta.
Respondé breve (1-2 frases), amable y directo, con tu personalidad.
NO inventes que tenés que ejecutar algo.
Si ya hay historial, NO saludes de nuevo.
Generá la respuesta ahora.`

	/* ---------- CLARIFY ---------- */
	case "clarify":
		var missing []string
		filled := map[string]interface{}{}
		for k, v := range req.DataSlots {
			if v == nil {
				missing = append(missing, k)
			} else {
				filled[k] = v
			}
		}

		if len(missing) > 0 {
			return base + fmt.Sprintf(`
## Tu tarea (PEDIR DATOS FALTANTES)
El usuario quiere: %s.
Ya dio: %s.
Falta: %s.

Generá UNA sola pregunta natural pidiendo SOLO los datos que faltan.
- No pidas lo que ya tiene.
- Sé conciso (1-2 frases).
- NO inventes alternativas.
Generá la pregunta ahora.`,
				req.Intent,
				formatMap(filled),
				strings.Join(missing, ", "),
			)
		}

		return base + `
## Tu tarea (PEDIR ACLARACIÓN)
El usuario dijo algo ambiguo. Pedí que reformule brevemente.
- Si el historial aclara, reformulá con eso: "¿Te referís a X?".
- NO inventes opciones.
Generá la respuesta ahora.`

	/* ---------- RESPOND ---------- */
	case "respond":
		return base + fmt.Sprintf(`
## Tu tarea (RESPONDER)
El sistema devolvió estos datos:

%s

## Cómo responder
- Redactá una respuesta natural usando el mensaje original, la intención y el campo "descripción".
- Copiá los números EXACTO, con puntos de mil: 126825000 → 126.825.000.
- Usá la unidad del campo "meta" si existe (ej: Gs).
- Usá SOLO lo que está en los datos. NO inventes.
- Breve (1-3 frases), con tu personalidad.
- No menciones términos técnicos ("JSON", "envelope", "sistema devolvió").

Generá la respuesta ahora.`,
			payload,
		)

	/* ---------- ERROR ---------- */
	case "error":
		return base + s.errorRules(req.ErrorCode)
	}

	return base
}

/* ============================================================
   ERROR RULES
============================================================ */
func (s *Synthesizer) errorRules(errorCode string) string {
	switch errorCode {
	case "execution_failed", "timeout":
		return `
## Tu tarea (FALLO DEL SISTEMA)
- Disculpate breve (1-2 frases), con tu personalidad.
- NO pidas datos nuevos.
- Sugerí intentar de nuevo en unos minutos.
- NO menciones detalles técnicos.
Generá la respuesta ahora.`

	default:
		return `
## Tu tarea (ERROR)
- Disculpate breve, con tu personalidad.
- NO pidas datos nuevos.
- Sugerí intentar de nuevo más tarde.
Generá la respuesta ahora.`
	}
}

/* ============================================================
   HELPERS
============================================================ */
func (s *Synthesizer) currentTime() time.Time {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		log.Printf("Synthesizer: timezone inválida %q, usando UTC", s.timezone)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
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