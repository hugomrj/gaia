// internal/normalizer/dates.go
package normalizer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- Detección ----------

var (
	reISODate       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reSlashDate     = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{2,4})$`)
	reDayMonth      = regexp.MustCompile(`^(\d{1,2})\s+de\s+([a-záéíóúñ]+)(?:\s+de\s+(\d{4}))?$`)
	reOffset        = regexp.MustCompile(`^hace\s+(\d+)\s+(días?|dias?|semanas?|meses?|años?|anos?)$`)
)

var relativeDays = map[string]int{
	"hoy":            0,
	"ayer":           -1,
	"anteayer":       -2,
	"antes de ayer":  -2,
	"mañana":         1,
	"manana":         1,
	"pasado mañana":  2,
	"pasado manana":  2,
}

var weekdays = map[string]time.Weekday{
	"domingo": time.Sunday,
	"lunes":   time.Monday,
	"martes":  time.Tuesday,
	"miércoles": time.Wednesday,
	"miercoles": time.Wednesday,
	"jueves":  time.Thursday,
	"viernes": time.Friday,
	"sábado":  time.Saturday,
	"sabado":  time.Saturday,
}

var monthNames = map[string]time.Month{
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

var periodKeywords = map[string]string{
	"esta semana":         "this_week",
	"la semana que viene": "next_week",
	"semana que viene":    "next_week",
	"la semana pasada":    "last_week",
	"semana pasada":       "last_week",
	"este mes":            "this_month",
	"el mes que viene":    "next_month",
	"mes que viene":       "next_month",
	"el mes pasado":       "last_month",
	"mes pasado":          "last_month",
	"este año":            "this_year",
	"este ano":            "this_year",
	"el año que viene":    "next_year",
	"año que viene":       "next_year",
	"el año pasado":       "last_year",
	"año pasado":          "last_year",
}

func looksLikeDate(s string) bool {
	if _, ok := relativeDays[s]; ok {
		return true
	}
	if _, ok := weekdays[s]; ok {
		return true
	}
	if reISODate.MatchString(s) || reSlashDate.MatchString(s) || reDayMonth.MatchString(s) {
		return true
	}
	if reOffset.MatchString(s) {
		return true
	}
	for k := range periodKeywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// ---------- Parseo ----------

// parseDate intenta interpretar la cadena como fecha simple (YYYY-MM-DD).
func parseDate(s string, now time.Time) (interface{}, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))

	// Relativos simples (futuro/pasado según tabla)
	if offset, ok := relativeDays[lower]; ok {
		return now.AddDate(0, 0, offset).Format("2006-01-02"), true
	}

	// "hace N días/meses/años"
	if m := reOffset.FindStringSubmatch(lower); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := m[2]
		switch {
		case strings.HasPrefix(unit, "día"), strings.HasPrefix(unit, "dia"):
			return now.AddDate(0, 0, -n).Format("2006-01-02"), true
		case strings.HasPrefix(unit, "semana"):
			return now.AddDate(0, 0, -7*n).Format("2006-01-02"), true
		case strings.HasPrefix(unit, "mes"):
			return now.AddDate(0, -n, 0).Format("2006-01-02"), true
		case strings.HasPrefix(unit, "año"), strings.HasPrefix(unit, "ano"):
			return now.AddDate(-n, 0, 0).Format("2006-01-02"), true
		}
	}

	// Día de la semana (siempre próximo)
	if wd, ok := weekdays[lower]; ok {
		return nextWeekday(now, wd).Format("2006-01-02"), true
	}
	if strings.HasPrefix(lower, "el ") {
		if wd, ok := weekdays[strings.TrimPrefix(lower, "el ")]; ok {
			return nextWeekday(now, wd).Format("2006-01-02"), true
		}
	}

	// ISO ya está listo
	if reISODate.MatchString(lower) {
		return lower, true
	}

	// DD/MM/YYYY o DD/MM/YY
	if m := reSlashDate.FindStringSubmatch(lower); m != nil {
		day, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if year < 100 {
			year += 2000
		}
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
		return t.Format("2006-01-02"), true
	}

	// "15 de octubre" o "15 de octubre de 2026"
	if m := reDayMonth.FindStringSubmatch(lower); m != nil {
		day, _ := strconv.Atoi(m[1])
		monthStr := m[2]
		year := now.Year()
		if m[3] != "" {
			year, _ = strconv.Atoi(m[3])
		}
		month, ok := monthNames[monthStr]
		if !ok {
			return nil, false
		}
		t := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
		return t.Format("2006-01-02"), true
	}

	return nil, false
}

// parsePeriod interpreta un período y devuelve {from, to}.
func parsePeriod(s string, now time.Time) (interface{}, bool) {
	key := ""
	for k := range periodKeywords {
		if strings.Contains(s, k) {
			key = periodKeywords[k]
			break
		}
	}
	if key == "" {
		return nil, false
	}

	var from, to time.Time

	switch key {
	case "this_week":
		from = startOfWeek(now)
		to = from.AddDate(0, 0, 6)
	case "next_week":
		from = startOfWeek(now).AddDate(0, 0, 7)
		to = from.AddDate(0, 0, 6)
	case "last_week":
		from = startOfWeek(now).AddDate(0, 0, -7)
		to = from.AddDate(0, 0, 6)
	case "this_month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = from.AddDate(0, 1, -1)
	case "next_month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
		to = from.AddDate(0, 1, -1)
	case "last_month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
		to = from.AddDate(0, 1, -1)
	case "this_year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		to = time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location())
	case "next_year":
		from = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, now.Location())
		to = time.Date(now.Year()+1, 12, 31, 0, 0, 0, 0, now.Location())
	case "last_year":
		from = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		to = time.Date(now.Year()-1, 12, 31, 0, 0, 0, 0, now.Location())
	default:
		return nil, false
	}

	return map[string]interface{}{
		"from": from.Format("2006-01-02"),
		"to":   to.Format("2006-01-02"),
	}, true
}

// ---------- Helpers ----------

func nextWeekday(now time.Time, target time.Weekday) time.Time {
	diff := int(target) - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	return now.AddDate(0, 0, diff)
}

func startOfWeek(now time.Time) time.Time {
	// Semana empieza el lunes
	wd := int(now.Weekday())
	offset := wd - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -offset)
}

// formatDate formatea a ISO sin hora.
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// helper por si querés imprimir
var _ = fmt.Sprintf