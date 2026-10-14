package normalizer

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	// 2026-10-13 (martes) 15:04:00 -03:00
	loc, _ := time.LoadLocation("America/Asuncion")
	return time.Date(2026, 10, 13, 15, 4, 0, 0, loc)
}

func TestDates(t *testing.T) {

	now := fixedNow()

	cases := []struct {
		in   string
		want string
	}{
		{"hoy", "2026-10-13"},
		{"ayer", "2026-10-12"},
		{"mañana", "2026-10-14"},
		{"pasado mañana", "2026-10-15"},
		{"anteayer", "2026-10-11"},
		{"el lunes", "2026-10-19"},
		{"el viernes", "2026-10-16"},
		{"15 de octubre", "2026-10-15"},
		{"15/10/2026", "2026-10-15"},
		{"15/10/26", "2026-10-15"},
		{"2026-10-15", "2026-10-15"},
		{"hace 5 días", "2026-10-08"},
		{"hace 3 meses", "2026-07-13"},
		{"hace 2 años", "2024-10-13"},
	}

	for _, c := range cases {
		got, ok := parseDate(c.in, now)
		if !ok {
			t.Errorf("parseDate(%q) no parseó", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("parseDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPeriods(t *testing.T) {
	now := fixedNow()

	cases := []struct {
		in       string
		fromWant string
		toWant   string
	}{
		{"esta semana", "2026-10-12", "2026-10-18"},
		{"la semana que viene", "2026-10-19", "2026-10-25"},
		{"este mes", "2026-10-01", "2026-10-31"},
		{"el mes que viene", "2026-11-01", "2026-11-30"},
		{"este año", "2026-01-01", "2026-12-31"},
		{"el año que viene", "2027-01-01", "2027-12-31"},
	}

	for _, c := range cases {
		got, ok := parsePeriod(c.in, now)
		if !ok {
			t.Errorf("parsePeriod(%q) no parseó", c.in)
			continue
		}
		m := got.(map[string]interface{})
		if m["from"] != c.fromWant || m["to"] != c.toWant {
			t.Errorf("parsePeriod(%q) = %+v, want from=%s to=%s",
				c.in, m, c.fromWant, c.toWant)
		}
	}
}

func TestNumbers(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"150000", int64(150000)},
		{"150 mil", int64(150000)},
		{"1.5M", 1500000.0},
		{"1,5M", 1500000.0},
		{"1500,50", 1500.5},
		{"1.500,50", 1500.5},
		{"1,500.50", 1500.5},
		{"-500", int64(-500)},
		{"150000 Gs.", int64(150000)},
		{"Gs. 150.000", int64(150000)},
		{"$1,500.50", 1500.5},
		{"€50", int64(50)},
	}

	for _, c := range cases {
		got, ok := parseNumber(c.in)
		if !ok {
			// Probar con parseMoney para casos con símbolo
			got, ok = parseMoney(c.in)
		}
		if !ok {
			t.Errorf("parseNumber(%q) no parseó", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("parseNumber(%q) = %v (%T), want %v (%T)",
				c.in, got, got, c.want, c.want)
		}
	}
}

func TestBooleans(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"sí", true},
		{"si", true},
		{"yes", true},
		{"true", true},
		{"1", true},
		{"no", false},
		{"false", false},
		{"0", false},
	}

	for _, c := range cases {
		got, ok := parseBoolean(c.in)
		if !ok {
			t.Errorf("parseBoolean(%q) no parseó", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("parseBoolean(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}