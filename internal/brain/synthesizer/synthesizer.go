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

var reMultiDot = regexp.MustCompile(`\.{2,}$`)
var reNumber   = regexp.MustCompile(`\d{4,}`) // Números de 4+ dígitos (los relevantes)

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

	now := s.currentTime()

	// 🛡️ BYPASS DE SEGURIDAD: Lógica fija, 0% alucinación, 0ms de latencia.
	if req.FunctionName == "obtener_fuera_alcance" {
		return &models.SynthesisResponse{
			Response:     "No puedo ayudarte con ese tema. Mi función es asistirte con facturación, clientes, stock o citas dentro del sistema.",
			HistoryEntry: buildHistoryEntry("conversation", req, now),
			Mode:         "conversation",
			DataSlots:    req.DataSlots,
			RawData:      req.RawData,
		}, nil
	}

	if req.ErrorCode == "function_not_found" {
		return &models.SynthesisResponse{
			Response:     "No puedo ayudarte con esa solicitud específica. Puedo asistirte con facturación, clientes, stock o citas.",
			HistoryEntry: buildHistoryEntry("error", req, now),
			Mode:         "error",
			DataSlots:    req.DataSlots,
			RawData:      req.RawData,
		}, nil
	}

	// 1. Determinar modo de forma directa y confiable
	mode := "respond" // Por defecto
	if req.IsConversational {
		mode = "conversation"
	} else if req.ErrorCode != "" {
		mode = "error"
	} else if req.RawData == "" {
		// Si no hay datos y hay slots nulos o baja confianza, es clarificar
		for _, v := range req.DataSlots {
			if v == nil {
				mode = "clarify"
				break
			}
		}
		if mode != "clarify" && req.Confidence < LowConfidenceThreshold {
			mode = "clarify"
		}
	}

	// 2. Payload limpio para el prompt
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

	// 3. Guard contra alucinación de números (solo en modo respond)
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





// guardNumbers verifica que el número MÁS IMPORTANTE del rawData esté en la respuesta.
func guardNumbers(content, rawData string) string {
	matches := reNumber.FindAllString(rawData, -1)
	if len(matches) == 0 {
		return content // No hay números grandes que proteger
	}

	targetNumber := matches[0]
	respDigits := regexp.MustCompile(`\D`).ReplaceAllString(content, "")

	if !strings.Contains(respDigits, targetNumber) {
		log.Printf("Synthesizer: ALERTA - Número principal %q no encontrado en respuesta. Respuesta: %q", targetNumber, content)
		formatted := formatNumber(targetNumber)
		return fmt.Sprintf("El resultado principal es %s.", formatted)
	}

	return content
}

func extractPayload(raw string) string {
	var env struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err == nil && env.Type != "" && len(env.Data) > 0 {
		res := strings.TrimSpace(string(env.Data))
		if strings.HasPrefix(res, "\"") && strings.HasSuffix(res, "\"") {
			return res[1 : len(res)-1]
		}
		return res
	}
	return raw
}

func formatNumber(s string) string {
	s = regexp.MustCompile(`\D`).ReplaceAllString(s, "")
	if len(s) <= 3 {
		return s
	}
	
	var out []byte
	for i, char := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(char))
	}
	return string(out)
}

func buildHistoryEntry(mode string, req models.SynthesisRequest, now time.Time) *models.HistoryEntry {
	var summary string
	switch mode {
	case "conversation":
		summary = fmt.Sprintf("respondí: %s", req.FunctionName)
	case "clarify":
		var missing []string
		for k, v := range req.DataSlots {
			if v == nil {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			summary = fmt.Sprintf("pedí datos faltantes: %s", strings.Join(missing, ", "))
		} else {
			summary = "pedí aclaración por baja confianza"
		}
	case "respond":
		summary = fmt.Sprintf("respondí sobre %s con datos del sistema", req.FunctionName)
	case "error":
		summary = fmt.Sprintf("informé error (%s) en %s", req.ErrorCode, req.FunctionName)
	default:
		summary = "respondí"
	}
	return &models.HistoryEntry{
		Role:      "assistant",
		Summary:   summary,
		Timestamp: now.Truncate(time.Second),
	}
}

func (s *Synthesizer) currentTime() time.Time {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		log.Printf("Synthesizer: timezone inválida %q, usando UTC", s.timezone)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
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
- Intención: %s
- Función: %s
- Parámetros: %s

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
Responde breve (1-2 frases), amable y directo.
- Si es saludo/despedida: sé cordial.
- ⚠️ PROHIBIDO el uso de emojis (😊, 👋, etc.). Usa solo texto profesional.
- Si pregunta "quién sos": respondé que sos el asistente del sistema empresarial.
NO inventes opciones ni hagas preguntas existenciales.
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
Ya proporcionó: %s.
Falta que proporcione: %s.

Genera UNA sola pregunta natural en español pidiendo SOLO los datos que faltan.
- Sé conciso (1-2 frases).
- Tono amable, sin sonar robótico.
- NO inventes alternativas ni opciones múltiples.
Genera la pregunta ahora.`,
				req.Intent,
				formatMap(filled),
				strings.Join(missing, ", "),
			)
		}

		return base + `
## Tu tarea (PEDIR ACLARACIÓN)
El usuario dijo algo ambiguo y no se pudo entender con certeza qué quiere.
- Si el historial aclara la intención, reformulá con eso: "¿Te referís a X?".
- Si no hay contexto, pedí que reformule brevemente.
- NO adivines ni inventes opciones.
Genera la respuesta ahora.`

	case "respond":
		return base + fmt.Sprintf(`
## Tu tarea (RESPONDER CON DATOS DEL SISTEMA)
El sistema devolvió estos DATOS para la petición del usuario:

%s

## REGLA CRÍTICA SOBRE NÚMEROS
- Los números en los datos DEBEN copiarse EXACTAMENTE, solo agregando puntos de mil.
- Si el dato dice "7075000", tu respuesta DEBE decir "7.075.000".
- NUNCA inventes, reordenes o elimines dígitos.
- Si el dato es "0" o "null", indicá que no hay resultados o el valor es cero.

## Reglas generales
- Usa SOLO los datos proporcionados. NO inventes.
- Si es una lista con más de 5 items, mostrá 5 y decí "y N más".
- Sé conciso (1-3 frases, o lista corta).
- No menciones "el sistema devolvió" ni términos técnicos como "JSON" o "envelope".
- Cierra con una pregunta simple solo si es natural: "¿Necesitás algo más?".

Genera la respuesta ahora.`,
			payload,
		)

	case "error":
		return base + s.errorRules(req.ErrorCode)
	}

	return base
}



func (s *Synthesizer) errorRules(errorCode string) string {
	switch errorCode {
	case "execution_failed", "timeout":
		return `
## Tu tarea (FALLO O DEMORA DEL SISTEMA)
- Discúlpate breve y claramente (1-2 frases).
- ⚠️ PROHIBIDO el uso de emojis (😊, 👋, etc.). Usa solo texto.
- NO pidas datos nuevos ni expliques el error técnico.
- Sugiere intentar de nuevo en unos minutos.
Ejemplo: "No pude completar la operación en este momento. ¿Podés intentar de nuevo en unos minutos?"
Genera la respuesta ahora.`

	default:
		return `
## Tu tarea (ERROR DEL SISTEMA)
- Discúlpate breve y claramente.
- ⚠️ PROHIBIDO el uso de emojis.
- NO pidas datos nuevos ni hagas preguntas sobre la operación.
- Sugiere intentar de nuevo más tarde.
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