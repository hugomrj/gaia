// internal/normalizer/dates.go
package normalizer

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- Detección ----------

var (
	reFechaISO      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reFechaBarra    = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{2,4})$`)
	reDiaMes        = regexp.MustCompile(`^(\d{1,2})\s+de\s+([a-záéíóúñ]+)(?:\s+de\s+(\d{4}))?$`)
	reDesplazamiento = regexp.MustCompile(`^hace\s+(\d+)\s+(días?|dias?|semanas?|meses?|años?|anos?)$`)
	reAnio          = regexp.MustCompile(`^\d{4}$`)
	reMesAnio       = regexp.MustCompile(`^([a-záéíóúñ]+)(?:\s+de\s+(\d{4}))?$`)
)

var diasRelativos = map[string]int{
	"hoy":           0,
	"ayer":          -1,
	"anteayer":      -2,
	"antes de ayer": -2,
	"mañana":        1,
	"manana":        1,
	"pasado mañana": 2,
	"pasado manana": 2,
}

var diasSemana = map[string]time.Weekday{
	"domingo":   time.Sunday,
	"lunes":     time.Monday,
	"martes":    time.Tuesday,
	"miércoles": time.Wednesday,
	"miercoles": time.Wednesday,
	"jueves":    time.Thursday,
	"viernes":   time.Friday,
	"sábado":    time.Saturday,
	"sabado":    time.Saturday,
}

var nombresMeses = map[string]time.Month{
	"enero":      time.January,
	"febrero":    time.February,
	"marzo":      time.March,
	"abril":      time.April,
	"mayo":       time.May,
	"junio":      time.June,
	"julio":      time.July,
	"agosto":     time.August,
	"septiembre": time.September,
	"setiembre":  time.September,
	"octubre":    time.October,
	"noviembre":  time.November,
	"diciembre":  time.December,
}

// palabrasPeriodo — slice ordenado de más específico a más genérico.
// Si dos entradas coinciden con el mismo texto, gana la primera.
var palabrasPeriodo = []struct {
	clave string
	tipo  string
}{
	// Mes
	{"el mes actual", "este_mes"},
	{"mes actual", "este_mes"},
	{"mes corriente", "este_mes"},
	{"este mes", "este_mes"},
	{"el mes que viene", "proximo_mes"},
	{"mes que viene", "proximo_mes"},
	{"el mes anterior", "mes_pasado"},
	{"mes anterior", "mes_pasado"},
	{"el mes pasado", "mes_pasado"},
	{"mes pasado", "mes_pasado"},

	// Semana
	{"esta semana", "esta_semana"},
	{"la semana que viene", "proxima_semana"},
	{"semana que viene", "proxima_semana"},
	{"la semana pasada", "semana_pasada"},
	{"semana pasada", "semana_pasada"},

	// Año
	{"año actual", "este_ano"},
	{"este año", "este_ano"},
	{"este ano", "este_ano"},
	{"el año que viene", "proximo_ano"},
	{"año que viene", "proximo_ano"},
	{"el año pasado", "ano_pasado"},
	{"año pasado", "ano_pasado"},
}

// pareceFecha indica si el texto podría ser una fecha o período.
func pareceFecha(s string) bool {
	if _, ok := diasRelativos[s]; ok {
		return true
	}
	if _, ok := diasSemana[s]; ok {
		return true
	}
	if reFechaISO.MatchString(s) || reFechaBarra.MatchString(s) || reDiaMes.MatchString(s) {
		return true
	}
	if reDesplazamiento.MatchString(s) {
		return true
	}
	if reAnio.MatchString(s) {
		return true
	}
	if reMesAnio.MatchString(s) {
		if _, ok := nombresMeses[s]; ok {
			return true
		}
	}
	for _, p := range palabrasPeriodo {
		if strings.Contains(s, p.clave) {
			return true
		}
	}
	return false
}

// ---------- Parseo ----------

// parsearFecha intenta interpretar la cadena como fecha simple (YYYY-MM-DD).
func parsearFecha(s string, ahora time.Time) (interface{}, bool) {
	min := strings.ToLower(strings.TrimSpace(s))

	if offset, ok := diasRelativos[min]; ok {
		return ahora.AddDate(0, 0, offset).Format("2006-01-02"), true
	}

	if m := reDesplazamiento.FindStringSubmatch(min); m != nil {
		n, _ := strconv.Atoi(m[1])
		unidad := m[2]
		switch {
		case strings.HasPrefix(unidad, "día"), strings.HasPrefix(unidad, "dia"):
			return ahora.AddDate(0, 0, -n).Format("2006-01-02"), true
		case strings.HasPrefix(unidad, "semana"):
			return ahora.AddDate(0, 0, -7*n).Format("2006-01-02"), true
		case strings.HasPrefix(unidad, "mes"):
			return ahora.AddDate(0, -n, 0).Format("2006-01-02"), true
		case strings.HasPrefix(unidad, "año"), strings.HasPrefix(unidad, "ano"):
			return ahora.AddDate(-n, 0, 0).Format("2006-01-02"), true
		}
	}

	if ds, ok := diasSemana[min]; ok {
		return proximoDiaSemana(ahora, ds).Format("2006-01-02"), true
	}
	if strings.HasPrefix(min, "el ") {
		if ds, ok := diasSemana[strings.TrimPrefix(min, "el ")]; ok {
			return proximoDiaSemana(ahora, ds).Format("2006-01-02"), true
		}
	}

	if reFechaISO.MatchString(min) {
		return min, true
	}

	if m := reFechaBarra.FindStringSubmatch(min); m != nil {
		dia, _ := strconv.Atoi(m[1])
		mes, _ := strconv.Atoi(m[2])
		anio, _ := strconv.Atoi(m[3])
		if anio < 100 {
			anio += 2000
		}
		t := time.Date(anio, time.Month(mes), dia, 0, 0, 0, 0, ahora.Location())
		return t.Format("2006-01-02"), true
	}

	if m := reDiaMes.FindStringSubmatch(min); m != nil {
		dia, _ := strconv.Atoi(m[1])
		mesStr := m[2]
		anio := ahora.Year()
		if m[3] != "" {
			anio, _ = strconv.Atoi(m[3])
		}
		mes, ok := nombresMeses[mesStr]
		if !ok {
			return nil, false
		}
		t := time.Date(anio, mes, dia, 0, 0, 0, 0, ahora.Location())
		return t.Format("2006-01-02"), true
	}

	return nil, false
}

// parsearPeriodo interpreta un período y devuelve {from, to}.
func parsearPeriodo(s string, ahora time.Time) (interface{}, bool) {
	min := strings.ToLower(strings.TrimSpace(s))

	// Año específico: "2026"
	if reAnio.MatchString(min) {
		anio, _ := strconv.Atoi(min)
		desde := time.Date(anio, 1, 1, 0, 0, 0, 0, ahora.Location())
		hasta := time.Date(anio, 12, 31, 0, 0, 0, 0, ahora.Location())
		return map[string]interface{}{
			"from": desde.Format("2006-01-02"),
			"to":   hasta.Format("2006-01-02"),
		}, true
	}

	// Mes específico: "octubre" o "octubre de 2026"
	if m := reMesAnio.FindStringSubmatch(min); m != nil {
		mesStr := m[1]
		anio := ahora.Year()
		if m[2] != "" {
			anio, _ = strconv.Atoi(m[2])
		}
		if mes, ok := nombresMeses[mesStr]; ok {
			desde := time.Date(anio, mes, 1, 0, 0, 0, 0, ahora.Location())
			hasta := desde.AddDate(0, 1, -1)
			return map[string]interface{}{
				"from": desde.Format("2006-01-02"),
				"to":   hasta.Format("2006-01-02"),
			}, true
		}
	}

	// Períodos con palabra clave
	tipo := ""
	for _, p := range palabrasPeriodo {
		if strings.Contains(min, p.clave) {
			tipo = p.tipo
			break
		}
	}
	if tipo == "" {
		return nil, false
	}

	var desde, hasta time.Time

	switch tipo {
	case "esta_semana":
		desde = inicioSemana(ahora)
		hasta = desde.AddDate(0, 0, 6)
	case "proxima_semana":
		desde = inicioSemana(ahora).AddDate(0, 0, 7)
		hasta = desde.AddDate(0, 0, 6)
	case "semana_pasada":
		desde = inicioSemana(ahora).AddDate(0, 0, -7)
		hasta = desde.AddDate(0, 0, 6)
	case "este_mes":
		desde = time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location())
		hasta = desde.AddDate(0, 1, -1)
	case "proximo_mes":
		desde = time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()).AddDate(0, 1, 0)
		hasta = desde.AddDate(0, 1, -1)
	case "mes_pasado":
		desde = time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()).AddDate(0, -1, 0)
		hasta = desde.AddDate(0, 1, -1)
	case "este_ano":
		desde = time.Date(ahora.Year(), 1, 1, 0, 0, 0, 0, ahora.Location())
		hasta = time.Date(ahora.Year(), 12, 31, 0, 0, 0, 0, ahora.Location())
	case "proximo_ano":
		desde = time.Date(ahora.Year()+1, 1, 1, 0, 0, 0, 0, ahora.Location())
		hasta = time.Date(ahora.Year()+1, 12, 31, 0, 0, 0, 0, ahora.Location())
	case "ano_pasado":
		desde = time.Date(ahora.Year()-1, 1, 1, 0, 0, 0, 0, ahora.Location())
		hasta = time.Date(ahora.Year()-1, 12, 31, 0, 0, 0, 0, ahora.Location())
	default:
		return nil, false
	}

	return map[string]interface{}{
		"from": desde.Format("2006-01-02"),
		"to":   hasta.Format("2006-01-02"),
	}, true
}

// ---------- Helpers ----------

func proximoDiaSemana(ahora time.Time, objetivo time.Weekday) time.Time {
	dif := int(objetivo) - int(ahora.Weekday())
	if dif <= 0 {
		dif += 7
	}
	return ahora.AddDate(0, 0, dif)
}

func inicioSemana(ahora time.Time) time.Time {
	dw := int(ahora.Weekday())
	offset := dw - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	return time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, ahora.Location()).AddDate(0, 0, -offset)
}