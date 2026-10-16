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
	ahora := n.horaActual()
	resultado := make(map[string]interface{}, len(params))
	var normalizados, noNormalizados []string

	for clave, crudo := range params {
		nuevoValor, cambio := n.normalizarValor(clave, crudo, ahora)
		resultado[clave] = nuevoValor
		if cambio {
			normalizados = append(normalizados, clave)
		} else {
			noNormalizados = append(noNormalizados, clave)
		}
	}

	return resultado, normalizados, noNormalizados
}

func (n *Normalizer) horaActual() time.Time {
	loc, err := time.LoadLocation(n.timezone)
	if err != nil {
		log.Printf("Normalizer: timezone inválida %q, usando UTC", n.timezone)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

// normalizarValor intenta normalizar un valor según el nombre del campo y su contenido.
func (n *Normalizer) normalizarValor(clave string, crudo interface{}, ahora time.Time) (interface{}, bool) {
	// Caso especial: cédula/RUC
	// - Si es string: se respeta tal cual (no se toca)
	// - Si es número: se convierte a string sin notación científica
	if esCampoCedula(clave) {
		if s, ok := crudo.(string); ok {
			return s, false // sin cambios
		}
		return numeroAString(crudo), true
	}

	str, esString := crudo.(string)
	if !esString {
		return crudo, false
	}

	recortado := strings.TrimSpace(str)
	if recortado == "" {
		return crudo, false
	}

	min := strings.ToLower(recortado)

	// 1. Booleanos (solo si el campo lo indica)
	if esCampoBooleano(clave) {
		if b, ok := parseBoolean(min); ok {
			return b, true
		}
	}

	// 2. Fechas / períodos
	if esCampoFecha(clave) || pareceFecha(min) {
		if v, ok := parsearFecha(recortado, ahora); ok {
			return v, true
		}
		if v, ok := parsearPeriodo(min, ahora); ok {
			return v, true
		}
	}

	// 3. Monedas / números
	if esCampoMoneda(clave) || pareceMoneda(recortado) {
		if v, ok := parsearMoneda(recortado); ok {
			return v, true
		}
	}
	if esCampoNumero(clave) || pareceNumero(recortado) {
		if v, ok := parsearNumero(recortado); ok {
			return v, true
		}
	}

	return crudo, false
}

// numeroAString convierte un número a string sin notación científica.
func numeroAString(v interface{}) string {
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

func esCampoFecha(clave string) bool {
	k := strings.ToLower(clave)
	campos := []string{
		"date", "fecha", "period", "periodo", "from", "desde",
		"to", "hasta", "start", "end", "inicio", "fin",
	}
	return contieneAlguno(k, campos)
}

func esCampoNumero(clave string) bool {
	k := strings.ToLower(clave)
	campos := []string{
		"amount", "monto", "cantidad", "quantity", "count", "qty",
		"price", "precio", "saldo", "balance", "cost", "costo",
	}
	return contieneAlguno(k, campos)
}

func esCampoMoneda(clave string) bool {
	k := strings.ToLower(clave)
	campos := []string{"total", "monto_total", "importe", "subtotal"}
	return contieneAlguno(k, campos)
}

func esCampoCedula(clave string) bool {
	k := strings.ToLower(clave)
	campos := []string{"cedula", "documento", "document", "ci", "ruc", "dni", "id_number"}
	return contieneAlguno(k, campos)
}

func esCampoBooleano(clave string) bool {
	k := strings.ToLower(clave)
	prefijos := []string{"is_", "has_", "tiene_", "es_", "esta_", "está_"}
	for _, p := range prefijos {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	exactos := []string{"active", "enabled", "activo", "habilitado"}
	return contieneAlguno(k, exactos)
}

func contieneAlguno(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}