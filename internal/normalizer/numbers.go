// internal/normalizer/numbers.go
package normalizer

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reMoney       = regexp.MustCompile(`(?i)(gs\.?|g\.?|usd|\$|€|£|₲)`)
	reNumeric     = regexp.MustCompile(`^-?[\d.,]+$`)
	reThousandK   = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mil|k)$`)
	reMillion     = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mill[oó]n(es)?|m)$`)
	reBillion     = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mil millones|b|bill[oó]n(es)?)$`)
)

func looksLikeMoney(s string) bool {
	return reMoney.MatchString(s)
}

func looksLikeNumber(s string) bool {
	s = strings.TrimSpace(s)
	if reNumeric.MatchString(s) {
		return true
	}
	if reThousandK.MatchString(s) || reMillion.MatchString(s) || reBillion.MatchString(s) {
		return true
	}
	return false
}

// parseMoney extrae el número de una cadena con símbolo de moneda.
func parseMoney(s string) (interface{}, bool) {
	clean := reMoney.ReplaceAllString(s, "")
	clean = strings.TrimSpace(clean)
	return parseNumber(clean)
}

// parseNumber interpreta cadenas como "150 mil", "1.5M", "1500,50", "150000".
func parseNumber(s string) (interface{}, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}

	// Multiplicadores
	if m := reMillion.FindStringSubmatch(s); m != nil {
		base, ok := parseNumber(m[1])
		if !ok {
			return nil, false
		}
		return multiply(base, 1_000_000), true
	}
	if m := reBillion.FindStringSubmatch(s); m != nil {
		base, ok := parseNumber(m[1])
		if !ok {
			return nil, false
		}
		return multiply(base, 1_000_000_000), true
	}
	if m := reThousandK.FindStringSubmatch(s); m != nil {
		base, ok := parseNumber(m[1])
		if !ok {
			return nil, false
		}
		return multiply(base, 1_000), true
	}

	// Detectar formato: 1.500,50 (europeo) o 1,500.50 (yanqui)
	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")

	normalized := s

	switch {
	case hasComma && hasDot:
		// El último separador indica el decimal
		if lastComma > lastDot {
			// Europeo: 1.500,50
			normalized = strings.ReplaceAll(s, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
		} else {
			// Yanqui: 1,500.50
			normalized = strings.ReplaceAll(s, ",", "")
		}
	case hasComma:
		// Solo coma: puede ser decimal (1500,50) o separador de miles (1,500)
		afterComma := s[lastComma+1:]
		if len(afterComma) == 3 && !strings.Contains(afterComma, ".") {
			// 1,500 → separador de miles
			normalized = strings.ReplaceAll(s, ",", "")
		} else {
			normalized = strings.ReplaceAll(s, ",", ".")
		}
	case hasDot:
		// Solo punto: puede ser decimal (1500.50) o separador de miles (1.500)
		afterDot := s[lastDot+1:]
		if len(afterDot) == 3 && !strings.Contains(afterDot, ",") {
			// 1.500 → separador de miles
			normalized = strings.ReplaceAll(s, ".", "")
		}
	}

	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return nil, false
	}

	// Si es entero, devolver int64; si no, float64
	if f == float64(int64(f)) {
		return int64(f), true
	}
	return f, true
}

func multiply(v interface{}, factor int64) interface{} {
	switch n := v.(type) {
	case int64:
		return n * factor
	case float64:
		return n * float64(factor)
	}
	return v
}