// internal/brain/synthesizer/synthesizer.go
package synthesizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gaia/internal/models"
	"gaia/internal/providers"
)

const (
	DefaultTimezone     = "America/Asuncion"
	MaxHistoryForPrompt = 6

	LowConfidenceThreshold = 0.7
)

var reMultiDot = regexp.MustCompile(`\.{2,}$`)

// reNonDigit matchea cualquier caracter que NO sea dígito.
var reNonDigit = regexp.MustCompile(`\D`)

// digitsOnly devuelve SOLO los dígitos de s.
// "7.075.000 Gs." → "7075000"
func digitsOnly(s string) string {
	return reNonDigit.ReplaceAllString(s, "")
}

// reNumber matchea un número (con separadores opcionales).
var reNumber = regexp.MustCompile(`\d[\d\s.,]*\d|\d`)

func extractFirstNumber(s string) string {
	return reNumber.FindString(s)
}

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

func (s *Synthesizer) Synthesize(
	ctx context.Context,
	req models.SynthesisRequest,
) (*models.SynthesisResponse, error) {

	mode := s.detectMode(req)
	now := s.currentTime()

	// Payload limpio (sin envelope) para el prompt y el guard
	payload := req.RawData
	if mode == "respond" && req.RawData != "" {
		payload = extractPayload(req.RawData)
	}

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

	// Guard contra el payload limpio, no contra el envelope
	if mode == "respond" && payload != "" {
		content = guardNumbers(content, payload)
	}

	return &models.SynthesisResponse{
		Response:     content,
		HistoryEntry: buildHistoryEntry(mode, req, now),
		Mode:         mode,
		DataSlots:    req.DataSlots,
		RawData:      req.RawData,
	}, nil
}

// guardNumbers verifica que los dígitos de rawData aparezcan en content.
func guardNumbers(content, rawData string) string {
	rawDigits := digitsOnly(rawData)
	if len(rawDigits) < 4 {
		return content
	}

	respDigits := digitsOnly(content)

	if !strings.Contains(respDigits, rawDigits) {
		log.Printf("Synthesizer: número inventado o perdido. raw_digits=%q response_digits=%q content=%q",
			rawDigits, respDigits, content)

		if n := extractFirstNumber(rawData); n != "" {
			return fmt.Sprintf("El resultado es %s.", formatNumber(digitsOnly(n)))
		}
		return content
	}

	return content
}

// extractPayload saca el dato útil del raw_data.
// Si es un envelope {"type":"single","data":X}, devuelve X como string.
func extractPayload(raw string) string {
	var env struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err == nil && env.Type != "" && len(env.Data) > 0 {
		return strings.Trim(string(env.Data), `"`)
	}
	return raw
}

// formatNumber agrega separadores de miles. "7075000" → "7.075.000"
func formatNumber(s string) string {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	str := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func (s *Synthesizer) detectMode(req models.SynthesisRequest) string {
	if strings.HasPrefix(req.Intent, "conversation") {
		return "conversation"
	}

	if req.ErrorCode != "" {
		return "error"
	}

	if req.Confidence > 0 && req.Confidence < LowConfidenceThreshold && req.RawData == "" {
		return "clarify"
	}

	if req.RawData != "" {
		if isEnvelopeError(req.RawData) {
			return "error"
		}
		if looksLikeError(req.RawData) {
			return "error"
		}
		return "respond"
	}

	for _, v := range req.DataSlots {
		if v == nil {
			return "clarify"
		}
	}

	return "conversation"
}

func isEnvelopeError(raw string) bool {
	compact := strings.ReplaceAll(raw, " ", "")
	return strings.Contains(compact, `"type":"error"`)
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

func buildHistoryEntry(mode string, req models.SynthesisRequest, now time.Time) *models.HistoryEntry {
	var summary string
	switch mode {
	case "conversation":
		if req.Intent == "conversation" {
			summary = "respondí charla casual"
		} else {
			summary = fmt.Sprintf("respondí %s", req.Intent)
		}
	case "clarify":
		var missing []string
		for k, v := range req.DataSlots {
			if v == nil {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			summary = fmt.Sprintf("pedí %s", strings.Join(missing, ", "))
		} else if req.Confidence > 0 && req.Confidence < LowConfidenceThreshold {
			summary = "pedí aclaración (confianza baja)"
		} else {
			summary = "pedí más información"
		}
	case "respond":
		summary = fmt.Sprintf("respondí sobre %s", req.Intent)
	case "error":
		if req.ErrorCode == "function_not_found" && req.FunctionName != "" {
			summary = fmt.Sprintf("función no encontrada: %s", req.FunctionName)
		} else if req.ErrorCode != "" {
			summary = fmt.Sprintf("error %s en %s", req.ErrorCode, req.Intent)
		} else {
			summary = fmt.Sprintf("error al procesar %s", req.Intent)
		}
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
	payload string,
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
El usuario hizo un saludo, una pregunta general o charla sin pedir una acción concreta.
Responde breve (1-2 frases), amable y directo.
NO inventes que tienes que ejecutar algo.
Si ya hay historial con mensajes previos, NO saludes de nuevo.

## Casos especiales
- Si pregunta "quién sos" → respondé con tu rol: asistente del sistema, sin inventar datos.
- Si pregunta "quién soy yo" → respondé: "Solo tengo acceso a los datos que me das en esta conversación. Si necesitás tu información, decime tu nombre o documento y te busco en el sistema."
- Si pregunta algo fuera del dominio del sistema (historia, clima, chistes) → decí amablemente que no podés ayudar con eso y ofrecé ayuda con lo que sí podés.

NO hagas preguntas existenciales ni filosóficas.
NO ofrezcas explorar la identidad, metas ni sentimientos del usuario.
NO inventes opciones ni alternativas que no estén en el contexto.

Genera la respuesta ahora.`

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
Falta que proporcione: %s.

Genera UNA sola pregunta natural en español pidiendo SOLO los datos que faltan.
- No pidas lo que ya tiene.
- No inventes datos.
- Sé conciso (1-2 frases).
- Tono amable, sin sonar robótico.
- NO hagas preguntas adicionales ni agregues opciones.
- NO inventes alternativas (no digas "¿es un cliente, una factura o un producto?" si no lo sabés).

Genera la pregunta ahora.`,
				req.Intent,
				formatMap(filled),
				strings.Join(missing, ", "),
			)
		}

		return base + `
## Tu tarea (PEDIR ACLARACIÓN)
El usuario dijo algo ambiguo y no se pudo entender con certeza qué quiere.

Reglas:
- NO inventes opciones (no digas "¿es un cliente o una factura?").
- NO adivines la intención.
- Si el historial aclara la intención, reformulá con eso: "¿Te referís a X?".
- Si no hay contexto, pedí que reformule.
- Sé breve (1-2 frases).
- Tono amable.

Ejemplos:
- Sin contexto: "No estoy seguro de qué necesitás. ¿Podés contarme un poco más?"
- Con contexto: "¿Te referís a la facturación de hoy?"

Genera la respuesta ahora.`

	case "respond":
		return base + fmt.Sprintf(`
## Tu tarea (RESPONDER CON DATOS DEL SISTEMA)
El usuario pidió: %s.
El sistema devolvió estos DATOS:

%s

## Cómo interpretar los datos
Los datos pueden venir en uno de estos formatos:

1. Envelope estándar (recomendado):
   {"type":"single","data":150000}
   {"type":"list","data":[...],"meta":{"count":3}}
   {"type":"empty","data":null}
   {"type":"error","data":null}

2. JSON libre: inferí la estructura y presentala naturalmente.

3. Texto plano: tomalo literal.

## CRITICAL: Numbers must be COPIED EXACTLY, never modified
- If data contains "7075000", your response MUST contain "7.075.000".
- Copy the number character by character. Do NOT invent, reorder, or drop digits.
- Examples:
  - "7075000"  → response must say "7.075.000" (NOT 1.707.500, NOT 707.500)
  - "9250000"  → response must say "9.250.000"
  - "150000"   → response must say "150.000"
- If you doubt, write the exact digits, then add separators.

## General rules
- Usa SOLO los datos proporcionados. NO inventes.
- Formatea números y fechas (ej: "150.000 Gs.").
- Si es una lista con más de 5 items, mostrá 5 y decí "y N más".
- Si meta.count existe, mencionalo ("encontré N...").
- Sé conciso (1-3 frases, o lista corta).
- No menciones "el sistema devolvió" ni cosas técnicas.
- NO hagas preguntas adicionales salvo un cierre simple tipo "¿Necesitás algo más?".

Genera la respuesta ahora.`,
			req.Intent,
			payload,
		)

	case "error":
		return base + s.errorRules(req.ErrorCode)
	}

	return base
}

func (s *Synthesizer) errorRules(errorCode string) string {
	switch errorCode {
	case "function_not_found":
		return `
## Tu tarea (FUERA DE ALCANCE)
El usuario pidió algo que el sistema NO puede hacer.
NO es un error técnico. Es simplemente que esa función no existe.

Reglas:
- NO digas "hubo un error" ni "problema técnico".
- Decí que no podés ayudar con eso específicamente.
- Ofrecé las áreas que sí manejás (según el contexto del sistema).
- Sé breve (1-2 frases).
- Tono amable.

Ejemplo: "No puedo ayudarte con eso desde acá. Puedo consultar sobre facturación, clientes, stock o citas. ¿Algo de eso?"

Genera la respuesta ahora.`

	case "execution_failed":
		return `
## Tu tarea (FALLO DEL SISTEMA)
El sistema intentó ejecutar la operación pero falló.

Reglas:
- Discúlpate breve y claramente (1-2 frases).
- NO pidas datos nuevos.
- Sugiere intentar de nuevo más tarde.
- NO menciones detalles técnicos.

Ejemplo: "No pude completar la operación en este momento. ¿Podés intentar de nuevo en unos minutos?"

Genera la respuesta ahora.`

	case "timeout":
		return `
## Tu tarea (DEMORA DEL SISTEMA)
El sistema está tardando más de lo esperado.

Reglas:
- Informá que hay demora.
- Sugiere reintentar en un momento.
- Sé breve (1-2 frases).
- NO menciones detalles técnicos.

Ejemplo: "El sistema está tardando más de lo esperado. ¿Podés intentar de nuevo en un momento?"

Genera la respuesta ahora.`

	default:
		return `
## Tu tarea (ERROR DEL SISTEMA)
El sistema falló al intentar ejecutar la operación.
El usuario NO debe enterarse del error técnico.

Reglas:
- Discúlpate breve y claramente (1-2 frases).
- NO pidas datos nuevos.
- NO hagas preguntas sobre la operación.
- Sugiere intentar de nuevo más tarde.

Ejemplo: "No pude completar la operación en este momento. ¿Podés intentar de nuevo en unos minutos?"

Genera la respuesta ahora.`
	}
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