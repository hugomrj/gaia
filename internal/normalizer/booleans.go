// internal/normalizer/booleans.go
package normalizer

import "strings"

var valoresVerdadero = map[string]bool{
	"sí": true, "si": true, "yes": true, "true": true, "1": true,
	"verdadero": true, "correcto": true, "ok": true,
}

var valoresFalso = map[string]bool{
	"no": true, "false": true, "0": true,
	"falso": true, "incorrecto": true,
}

// parseBoolean interpreta cadenas como "sí", "no", "true", "false".
func parseBoolean(s string) (bool, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if valoresVerdadero[s] {
		return true, true
	}
	if valoresFalso[s] {
		return false, true
	}
	return false, false
}