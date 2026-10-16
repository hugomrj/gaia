// internal/normalizer/numbers.go
package normalizer

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reMoneda      = regexp.MustCompile(`(?i)(gs\.?|g\.?|usd|\$|€|£|₲)`)
	reNumerico    = regexp.MustCompile(`^-?[\d.,]+$`)
	reMilK        = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mil|k)$`)
	reMillon      = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mill[oó]n(es)?|m)$`)
	reMilMillones = regexp.MustCompile(`(?i)^(-?[\d.,]+)\s*(mil millones|b|bill[oó]n(es)?)$`)
)

// pareceMoneda indica si el texto contiene un símbolo de moneda.
func pareceMoneda(s string) bool {
	return reMoneda.MatchString(s)
}

// pareceNumero indica si el texto parece un número (con o sin multiplicador).
func pareceNumero(s string) bool {
	s = strings.TrimSpace(s)
	if reNumerico.MatchString(s) {
		return true
	}
	if reMilK.MatchString(s) || reMillon.MatchString(s) || reMilMillones.MatchString(s) {
		return true
	}
	return false
}

// parsearMoneda extrae el número de una cadena con símbolo de moneda.
func parsearMoneda(s string) (interface{}, bool) {
	limpio := reMoneda.ReplaceAllString(s, "")
	limpio = strings.TrimSpace(limpio)
	return parsearNumero(limpio)
}

// parsearNumero interpreta cadenas como "150 mil", "1.5M", "1500,50", "150000".
func parsearNumero(s string) (interface{}, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}

	// Multiplicadores
	if m := reMillon.FindStringSubmatch(s); m != nil {
		base, ok := parsearNumero(m[1])
		if !ok {
			return nil, false
		}
		return multiplicar(base, 1_000_000), true
	}
	if m := reMilMillones.FindStringSubmatch(s); m != nil {
		base, ok := parsearNumero(m[1])
		if !ok {
			return nil, false
		}
		return multiplicar(base, 1_000_000_000), true
	}
	if m := reMilK.FindStringSubmatch(s); m != nil {
		base, ok := parsearNumero(m[1])
		if !ok {
			return nil, false
		}
		return multiplicar(base, 1_000), true
	}

	// Detectar formato: 1.500,50 (europeo) o 1,500.50 (yanqui)
	tieneComa := strings.Contains(s, ",")
	tienePunto := strings.Contains(s, ".")
	ultimaComa := strings.LastIndex(s, ",")
	ultimoPunto := strings.LastIndex(s, ".")

	normalizado := s

	switch {
	case tieneComa && tienePunto:
		// El último separador indica el decimal
		if ultimaComa > ultimoPunto {
			// Europeo: 1.500,50
			normalizado = strings.ReplaceAll(s, ".", "")
			normalizado = strings.ReplaceAll(normalizado, ",", ".")
		} else {
			// Yanqui: 1,500.50
			normalizado = strings.ReplaceAll(s, ",", "")
		}
	case tieneComa:
		// Solo coma: puede ser decimal (1500,50) o separador de miles (1,500)
		despuesComa := s[ultimaComa+1:]
		if len(despuesComa) == 3 && !strings.Contains(despuesComa, ".") {
			// 1,500 → separador de miles
			normalizado = strings.ReplaceAll(s, ",", "")
		} else {
			normalizado = strings.ReplaceAll(s, ",", ".")
		}
	case tienePunto:
		// Solo punto: puede ser decimal (1500.50) o separador de miles (1.500)
		despuesPunto := s[ultimoPunto+1:]
		if len(despuesPunto) == 3 && !strings.Contains(despuesPunto, ",") {
			// 1.500 → separador de miles
			normalizado = strings.ReplaceAll(s, ".", "")
		}
	}

	f, err := strconv.ParseFloat(normalizado, 64)
	if err != nil {
		return nil, false
	}

	// Si es entero, devolver int64; si no, float64
	if f == float64(int64(f)) {
		return int64(f), true
	}
	return f, true
}

// multiplicar aplica un factor a un número (int64 o float64).
func multiplicar(v interface{}, factor int64) interface{} {
	switch n := v.(type) {
	case int64:
		return n * factor
	case float64:
		return n * float64(factor)
	}
	return v
}