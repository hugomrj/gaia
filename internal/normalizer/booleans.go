// internal/normalizer/booleans.go
package normalizer

import "strings"

var trueValues = map[string]bool{
	"sí": true, "si": true, "yes": true, "true": true, "1": true,
	"verdadero": true, "correcto": true, "ok": true,
}

var falseValues = map[string]bool{
	"no": true, "false": true, "0": true,
	"falso": true, "incorrecto": true,
}

func parseBoolean(s string) (bool, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if trueValues[s] {
		return true, true
	}
	if falseValues[s] {
		return false, true
	}
	return false, false
}