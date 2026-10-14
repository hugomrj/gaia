// internal/normalizer/normalizer.go
package normalizer

import (
	"fmt"
	"log"
	"strconv"  
	"strings"
	"time"
)

const DefaultTimezone = "America/Asuncion"

// Normalizer aplica reglas determinísticas a los parámetros.
// No usa LLM. Solo transforma valores reconocibles.
type Normalizer struct {
	timezone string
}

func New() *Normalizer {
	return &Normalizer{timezone: DefaultTimezone}
}

func (n *Normalizer) SetTimezone(tz string) {
	if tz != "" {
		n.timezone = tz
	}
}

// Normalize toma un map de parámetros y devuelve un nuevo map con los valores
// normalizados cuando sea posible. No modifica el original.
func (n *Normalizer) Normalize(params map[string]interface{}) (map[string]interface{}, []string, []string) {
	now := n.currentTime()
	result := make(map[string]interface{}, len(params))
	var normalized, notNormalized []string

	for key, raw := range params {
		newValue, changed := n.normalizeValue(key, raw, now)
		result[key] = newValue
		if changed {
			normalized = append(normalized, key)
		} else {
			notNormalized = append(notNormalized, key)
		}
	}

	return result, normalized, notNormalized
}

func (n *Normalizer) currentTime() time.Time {
	loc, err := time.LoadLocation(n.timezone)
	if err != nil {
		log.Printf("Normalizer: timezone inválida %q, usando UTC", n.timezone)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

// normalizeValue intenta normalizar un valor según el nombre del campo y su contenido.
func (n *Normalizer) normalizeValue(key string, raw interface{}, now time.Time) (interface{}, bool) {
	// Caso especial: cédula/RUC
	// - Si es string: se respeta tal cual (no se toca)
	// - Si es número: se convierte a string sin notación científica
	if isCedulaField(key) {
		if s, ok := raw.(string); ok {
			return s, false // sin cambios
		}
		return numberToString(raw), true
	}

	str, isString := raw.(string)
	if !isString {
		return raw, false
	}

	trimmed := strings.TrimSpace(str)
	if trimmed == "" {
		return raw, false
	}

	lower := strings.ToLower(trimmed)

	// 1. Booleanos (solo si el campo lo indica)
	if isBooleanField(key) {
		if b, ok := parseBoolean(lower); ok {
			return b, true
		}
	}

	// 2. Fechas / períodos
	if isDateField(key) || looksLikeDate(lower) {
		if v, ok := parseDate(trimmed, now); ok {
			return v, true
		}
		if v, ok := parsePeriod(lower, now); ok {
			return v, true
		}
	}

	// 3. Monedas / números
	if isMoneyField(key) || looksLikeMoney(trimmed) {
		if v, ok := parseMoney(trimmed); ok {
			return v, true
		}
	}
	if isNumberField(key) || looksLikeNumber(trimmed) {
		if v, ok := parseNumber(trimmed); ok {
			return v, true
		}
	}

	return raw, false
}



// numberToString convierte un número a string sin notación científica.
func numberToString(v interface{}) string {
	switch n := v.(type) {
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case float64:
		if n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}


// ---------- Hints por nombre de campo ----------

func isDateField(key string) bool {
	k := strings.ToLower(key)
	fields := []string{
		"date", "fecha", "period", "periodo", "from", "desde",
		"to", "hasta", "start", "end", "inicio", "fin",
	}
	return containsAny(k, fields)
}

func isNumberField(key string) bool {
	k := strings.ToLower(key)
	fields := []string{
		"amount", "monto", "cantidad", "quantity", "count", "qty",
		"price", "precio", "saldo", "balance", "cost", "costo",
	}
	return containsAny(k, fields)
}

func isMoneyField(key string) bool {
	k := strings.ToLower(key)
	fields := []string{"total", "monto_total", "importe", "subtotal"}
	return containsAny(k, fields)
}

func isCedulaField(key string) bool {
	k := strings.ToLower(key)
	fields := []string{"cedula", "documento", "document", "ci", "ruc", "dni", "id_number"}
	return containsAny(k, fields)
}

func isBooleanField(key string) bool {
	k := strings.ToLower(key)
	prefixes := []string{"is_", "has_", "tiene_", "es_", "esta_", "está_"}
	for _, p := range prefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	exact := []string{"active", "enabled", "activo", "habilitado"}
	return containsAny(k, exact)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}