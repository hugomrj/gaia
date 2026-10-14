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
	DefaultTimezone     = "America/Asuncion"
	MaxHistoryForPrompt = 6
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

func (p *Planner) SetTimezone(tz string) {
	if tz != "" {
		p.timezone = tz
	}
}

func (p *Planner) Analyze(
	ctx context.Context,
	req models.ChatRequest,
) (*models.AnalysisResponse, error) {

	now := p.currentTime()
	systemPrompt := p.buildSystemPrompt(req, now)

	log.Printf("[Planner] PROMPT ↓")
	log.Println(systemPrompt)
	log.Printf("[Planner] PROMPT ↑")

	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Message},
	}

	tools := p.getAnalysisTools()

	llmResponse, err := p.llm.Chat(ctx, messages, tools)
	if err != nil {
		return nil, fmt.Errorf("error al llamar al LLM: %w", err)
	}

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
			analysis.IsConversational = toBool(args["is_conversational"])
			analysis.FunctionName = toString(args["function_name"])
			analysis.Parameters = toMap(args["parameters"])
			analysis.Confidence = toFloat64(args["confidence"])
		}
	} else {
		analysis.Intent = "El usuario no dio una instrucción clara"
		analysis.IsConversational = true
		analysis.FunctionName = "obtener_conversacion"
		analysis.Parameters = map[string]interface{}{}
		analysis.Confidence = 0.3
	}

	return analysis, nil
}

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

	slotsStr := "(ninguno)"
	if len(req.DataSlots) > 0 {
		parts := make([]string, 0, len(req.DataSlots))
		for k, v := range req.DataSlots {
			if v == nil {
				parts = append(parts, fmt.Sprintf("%s=null", k))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
		}
		slotsStr = "{" + strings.Join(parts, ", ") + "}"
	}

	functionStr := req.FunctionName
	if functionStr == "" {
		functionStr = "(ninguna)"
	}

	historyStr := p.formatHistory(req.History)

	return fmt.Sprintf(`Sos GAIA. Leé el mensaje del usuario y devolvé un JSON vía update_intent.

## Contexto
- Fecha: %s (%s)
- Función pendiente: %s
- data_slots: %s
- Historial:
%s

## Salida
{
  "intent": "frase en castellano que resume qué quiere el usuario",
  "is_conversational": false,
  "function_name": "verbo_sustantivo[_sufijo]",
  "parameters": {},
  "confidence": 0.0
}

## PRIORIDAD 1 — data_slots pendientes

Si "Función pendiente" NO es "(ninguna)" y hay slots en null:
  → Tu tarea es SOLO asignar el valor del mensaje al slot que encaje.
  → MANTENÉ la función pendiente. NO la cambies.
  → Usá la CLAVE EXACTA del slot.
  → Valor CRUDO: "el lunes" queda "el lunes". No normalices.
  → is_conversational: false.
  → Solo cambiá de función si el usuario EXPLÍCITAMENTE pide otra cosa
    ("mejor dame X", "ahora quiero Y", "olvidalo, dame Z").

Si NO hay función pendiente, ignorá esta sección y seguí con PRIORIDAD 2.

## PRIORIDAD 2 — Clasificación

- CONVERSACIÓN (saludo, gracias, despedida, "quien sos", "mi perfil"):
  → is_conversational: true
  → obtener_saludo | obtener_agradecimiento | obtener_despedida | obtener_info_bot | obtener_mi_perfil
  → confidence 1.0.
- TOPIC (solo un sustantivo, sin acción ni criterio): "facturacion", "clientes".
  → is_conversational: false
  → function_name "", parameters {"tema":"<topic>"}, confidence 0.5.
- PETICIÓN: cualquier otra cosa (verbo, pregunta, criterio específico).
  → is_conversational: false
- AMBIGUO sin contexto ("dame eso", "che dame eso"):
  → is_conversational: false
  → obtener_desconocido, {}, 0.3.
- FUERA DE ALCANCE (no es del sistema, "quien descubrio america"):
  → is_conversational: false
  → obtener_fuera_alcance, {}, 0.9.

## PRIORIDAD 3 — Verbo

Verbo (uno solo, en infinitivo):
obtener_, listar_, buscar_, verificar_, crear_, actualizar_, eliminar_,
calcular_, comparar_, generar_, enviar_, agendar_.

| Verbo | Cuándo |
|---|---|
| obtener_ + SINGULAR | un valor o reporte (incluye reportes con cálculo adentro: margen, cuentas por cobrar, saldo, estado de resultados, top clientes, KPIs, facturación total, posición de caja) |
| listar_ + PLURAL | múltiples registros sin filtro |
| buscar_ + PLURAL | múltiples con filtro (ciudad, categoría) |
| verificar_ + SINGULAR | existencia/estado (sí/no) |
| calcular_ | SOLO operación matemática explícita entre dos valores: "cuánto crecí en porcentaje", "promedio", "diferencia". SI DUDÁS, USÁ obtener_ o comparar_. |
| comparar_ | comparar dos períodos o valores ("vs", "respecto a", "crecí") |
| generar_ | producir artefacto ("generame", "armá", "creá un informe/reporte/PDF") |
| crear_, actualizar_, eliminar_, enviar_, agendar_ | acciones directas |

Regla de desempate: si dudás entre calcular_ y obtener_/comparar_, elegí obtener_/comparar_. calcular_ es raro.

## PRIORIDAD 4 — Sustantivo

Vocabulario CERRADO. Usá el término EXACTO. NUNCA sinónimos.

facturacion, clientes, productos, vendedores, categorias, ciudad,
saldos, caja, posicion, cuentas, margen, kpis, proveedores, empleados,
departamentos, inventario, stock, ordenes, reuniones, resultados,
tareas, temas, analisis, informes, reportes.

Equivalencias (siempre estas, nunca otras):
- "facturación" = "ventas" = "ingresos" → facturacion
- "saldo" = "deuda" → saldos
- "caja" = "efectivo" = "liquidez" → posicion (cuando es "posicion de caja")
- "informe" = "reporte" → informes

Si no está en la lista, elegí el más cercano. NUNCA inventes.

## PRIORIDAD 5 — Forma del nombre

- verbo_sustantivo[_sufijo]. MÁXIMO 3 partes.
- PROHIBIDAS preposiciones (por/de/para). Convertí a sufijo.
  ✅ obtener_facturacion_mensual
  ❌ obtener_facturacion_por_mes
- Sufijos:
  - Período: _diaria, _semanal, _mensual, _anual, _periodo
  - Agrupación: _vendedor, _categoria, _ciudad, _cliente, _producto
- Los sufijos de agrupación son SOLO para agrupar, NO para filtrar.
  Filtros van en parameters.
  ✅ "clientes de asuncion" → buscar_clientes {ciudad:"asuncion"}
  ❌ "clientes de asuncion" → buscar_clientes_ciudad

## PRIORIDAD 6 — Períodos

- Día ("hoy", "ayer", "el lunes") → obtener_facturacion_diaria, {"fecha":"<crudo>"}
- Mes/semana/año singular ("este mes", "el mes pasado") → obtener_facturacion_periodo, {"periodo":"<crudo>"}
- Año específico ("2026") → obtener_facturacion_periodo, {"periodo":"2026"}
- Plural/desglose ("por meses", "por vendedor") → sufijo.
  "facturacion por meses" → obtener_facturacion_mensual, {"meses":6}

## PRIORIDAD 7 — Parámetros

- Claves SOLO de: fecha, periodo, meses, cliente, ciudad, producto, vendedor,
  monto, cantidad, proveedor, estado, limite, sucursal, tema.
- Valores CRUDOS, sin normalizar. Años como STRING ("2026").
- No inventes claves. Si algo no encaja, no lo extraigas.
- Si no necesita parámetros → {}.

## PRIORIDAD 8 — Confianza

- Completo → 0.95-1.0
- Ambigüedad menor → 0.85-0.95
- Falta dato → 0.6-0.8
- Topic → 0.5
- Ambiguo → 0.3
- Fuera de alcance → 0.9
Typos NO bajan la confianza.

## Ejemplos
- "hola" → {"intent":"El usuario saluda","is_conversational":true,"function_name":"obtener_saludo","parameters":{},"confidence":1.0}
- "cuanto facture hoy" → {"intent":"El usuario quiere saber cuánto se facturó hoy","is_conversational":false,"function_name":"obtener_facturacion_diaria","parameters":{"fecha":"hoy"},"confidence":0.95}
- "cuanto facture este mes" → {"intent":"El usuario quiere saber cuánto se facturó este mes","is_conversational":false,"function_name":"obtener_facturacion_periodo","parameters":{"periodo":"este mes"},"confidence":0.95}
- "dame la facturacion total 2026" → {"intent":"El usuario quiere la facturación del año 2026","is_conversational":false,"function_name":"obtener_facturacion_periodo","parameters":{"periodo":"2026"},"confidence":0.95}
- "ventas por vendedor" → {"intent":"El usuario quiere ver las ventas por vendedor","is_conversational":false,"function_name":"listar_facturacion_vendedor","parameters":{},"confidence":0.95}
- "creci respecto al mes pasado" → {"intent":"El usuario quiere comparar la facturación contra el mes pasado","is_conversational":false,"function_name":"comparar_facturacion_periodos","parameters":{"periodo":"el mes pasado"},"confidence":0.95}
- "posicion de caja" → {"intent":"El usuario quiere ver la posición de caja","is_conversational":false,"function_name":"obtener_posicion_caja","parameters":{},"confidence":0.95}
- "cuentas por cobrar" → {"intent":"El usuario quiere ver las cuentas por cobrar","is_conversational":false,"function_name":"obtener_cuentas_cobrar","parameters":{},"confidence":0.95}
- "generame un reporte de ventas" → {"intent":"El usuario quiere generar un informe de ventas","is_conversational":false,"function_name":"generar_informe_ventas","parameters":{},"confidence":0.95}
- (pendiente: agendar_reunion, slots {doctor:"ramirez", fecha:null}) "el lunes" → {"intent":"El usuario responde con la fecha para la reunión","is_conversational":false,"function_name":"agendar_reunion","parameters":{"fecha":"el lunes"},"confidence":0.95}
- (pendiente: agendar_reunion, slots {doctor:"ramirez", fecha:null}) "mejor dame la facturacion del mes" → {"intent":"El usuario cambia de tema y pide la facturación del mes","is_conversational":false,"function_name":"obtener_facturacion_periodo","parameters":{"periodo":"este mes"},"confidence":0.95}
- "facturacion" → {"intent":"El usuario menciona el tema facturación sin acción clara","is_conversational":false,"function_name":"","parameters":{"tema":"facturacion"},"confidence":0.5}
- "che dame eso" → {"intent":"El usuario hace una referencia ambigua","is_conversational":false,"function_name":"obtener_desconocido","parameters":{},"confidence":0.3}

## Mensaje actual
"%s"

Devolvé vía update_intent.`,
		now.Format("2006-01-02 15:04:05"),
		p.timezone,
		functionStr,
		slotsStr,
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
				Description: "Extrae intent, is_conversational, function_name, parameters y confidence del mensaje del usuario",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"intent": map[string]interface{}{
							"type":        "string",
							"description": "UNA FRASE en castellano que resuma qué quiere hacer el usuario. Sin snake_case, sin límite de palabras.",
						},
						"is_conversational": map[string]interface{}{
							"type":        "boolean",
							"description": "true si el mensaje es charla casual (saludo, gracias, despedida, info del bot, mi perfil). false en cualquier otro caso.",
						},
						"function_name": map[string]interface{}{
							"type":        "string",
							"description": "Nombre de función: verbo_sustantivo[_sufijo] en castellano. Verbo: obtener_, listar_, buscar_, verificar_, crear_, actualizar_, eliminar_, calcular_, comparar_, generar_, enviar_, agendar_. Sustantivo: solo del vocabulario permitido (facturacion, clientes, productos, vendedores, categorias, ciudad, saldos, caja, posicion, cuentas, margen, kpis, proveedores, empleados, departamentos, inventario, stock, ordenes, reuniones, resultados, tareas, temas, analisis, informes, reportes). Máximo 3 partes, sin preposiciones. String vacío si es un topic sin acción.",
						},
						"parameters": map[string]interface{}{
							"type":        "object",
							"description": "Claves SOLO de: fecha, periodo, meses, cliente, ciudad, producto, vendedor, monto, cantidad, proveedor, estado, limite, sucursal, tema. Valores CRUDOS sin normalizar. Años como STRING (\"2026\"). Si no necesita parámetros: {}.",
						},
						"confidence": map[string]interface{}{
							"type":        "number",
							"description": "0.0 a 1.0. Completo → 0.95. Topic → 0.5. Vago → 0.3.",
						},
					},
					"required": []string{"intent", "is_conversational", "function_name", "parameters", "confidence"},
				},
			},
		},
	}
}

func (p *Planner) formatHistory(history []models.HistoryEntry) string {
	if len(history) == 0 {
		return "(sin historial previo)"
	}

	start := 0
	if len(history) > MaxHistoryForPrompt {
		start = len(history) - MaxHistoryForPrompt
	}

	var sb strings.Builder
	for _, entry := range history[start:] {
		label := "usuario"
		content := entry.Content
		if entry.Role == "assistant" {
			label = "asistente"
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

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}