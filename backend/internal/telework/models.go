package telework

import "time"

// Preset defines the default weekly pattern: which weekdays are remote.
// Weekdays: 0=Sunday, 1=Monday, ..., 6=Saturday
type Preset struct {
	RemoteDays []int `json:"remote_days"` // weekday numbers that are TT by default
}

// Override represents a per-day override of the weekly preset.
// Type is one of: "leave", "remote", "office".
type Override struct {
	Date string `json:"date"` // YYYY-MM-DD
	Type string `json:"type"`
}

// Holiday represents a public holiday in Geneva.
type Holiday struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Label string `json:"label"` // human-readable name
}

// DaySummary describes a single calendar day.
type DaySummary struct {
	Date         string `json:"date"`
	Weekday      int    `json:"weekday"` // 0=Sun..6=Sat
	IsWeekend    bool   `json:"is_weekend"`
	IsHoliday    bool   `json:"is_holiday"`
	IsLeave      bool   `json:"is_leave"`
	IsRemote     bool   `json:"is_remote"`      // TT day (preset or override)
	OverrideType string `json:"override_type"`  // "leave"|"remote"|"office"|"" — explicit override set by user
}

// YearSummary is the full payload returned to the frontend.
type YearSummary struct {
	Year          int          `json:"year"`
	Days          []DaySummary `json:"days"`
	Holidays      []Holiday    `json:"holidays"`
	TotalWorked   int          `json:"total_worked"`   // worked = not weekend, not holiday, not leave
	TotalRemote   int          `json:"total_remote"`   // remote among worked days
	TotalOnSite   int          `json:"total_on_site"`  // on-site among worked days
	RemotePct     float64      `json:"remote_pct"`     // TotalRemote / TotalWorked * 100
	OnSitePct     float64      `json:"on_site_pct"`
	DaysToRecover int          `json:"days_to_recover"` // on-site days needed to reach 60% on-site (= ≤40% TT)
	OverThreshold bool         `json:"over_threshold"`  // remote_pct > 40
}

// genevaHolidays returns public holidays for Geneva for a given year, computed dynamically.
func genevaHolidays(year int) []Holiday {
	holidays := []Holiday{
		{date(year, 1, 1), "Nouvel An"},
		{date(year, 12, 25), "Noël"},
		{date(year, 12, 31), "Restauration genevoise"},
		{date(year, 8, 1), "Fête nationale"},
	}

	// Easter-based holidays (Meeus/Jones/Butcher algorithm)
	easter := easterDate(year)
	goodFriday := easter.AddDate(0, 0, -2)
	easterMonday := easter.AddDate(0, 0, 1)
	ascension := easter.AddDate(0, 0, 39)
	whitSunday := easter.AddDate(0, 0, 49)
	whitMonday := easter.AddDate(0, 0, 50)

	holidays = append(holidays,
		Holiday{goodFriday.Format("2006-01-02"), "Vendredi Saint"},
		Holiday{easter.Format("2006-01-02"), "Pâques"},
		Holiday{easterMonday.Format("2006-01-02"), "Lundi de Pâques"},
		Holiday{ascension.Format("2006-01-02"), "Ascension"},
		Holiday{whitSunday.Format("2006-01-02"), "Pentecôte"},
		Holiday{whitMonday.Format("2006-01-02"), "Lundi de Pentecôte"},
	)

	// Jeûne genevois: first Thursday after the first Sunday of September
	jeune := jeuneGenevois(year)
	holidays = append(holidays, Holiday{jeune.Format("2006-01-02"), "Jeûne genevois"})

	return holidays
}

func date(year, month, day int) string {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// easterDate computes Easter Sunday using the Meeus/Jones/Butcher algorithm.
func easterDate(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// jeuneGenevois computes the Jeûne genevois: Thursday after the first Sunday in September.
func jeuneGenevois(year int) time.Time {
	sep1 := time.Date(year, time.September, 1, 0, 0, 0, 0, time.UTC)
	// Find first Sunday in September
	daysUntilSunday := (7 - int(sep1.Weekday())) % 7
	firstSunday := sep1.AddDate(0, 0, daysUntilSunday)
	// Thursday after first Sunday
	return firstSunday.AddDate(0, 0, 4)
}
