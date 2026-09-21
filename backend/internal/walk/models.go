package walk

// Entry is the distance walked on a single day.
type Entry struct {
	Date string  `json:"date"` // YYYY-MM-DD
	Km   float64 `json:"km"`
}

// Day describes a calendar day in the year grid.
type Day struct {
	Date     string  `json:"date"`    // YYYY-MM-DD
	Weekday  int     `json:"weekday"` // 0=Sun..6=Sat
	Km       float64 `json:"km"`
	IsFuture bool    `json:"is_future"`
}

// YearSummary is the full payload returned to the frontend.
// WeekTotal and MonthTotal always refer to the current calendar week/month,
// independently of the requested year.
type YearSummary struct {
	Year        int       `json:"year"`
	Days        []Day     `json:"days"`
	WeekTotal   float64   `json:"week_total"`
	MonthTotal  float64   `json:"month_total"`
	YearTotal   float64   `json:"year_total"`
	MonthTotals []float64 `json:"month_totals"` // 12 entries, index 0 = January
	ActiveDays  int       `json:"active_days"`
	DailyAvg    float64   `json:"daily_avg"` // average over active days only
	BestDay     Entry     `json:"best_day"`
}
