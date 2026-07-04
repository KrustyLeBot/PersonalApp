package telework

import (
	"math"
	"time"
)

// Service computes the telework summary for a given year.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// ComputeYear builds a YearSummary for the given year and user.
func (s *Service) ComputeYear(year int, email string) (YearSummary, error) {
	preset, err := s.repo.GetPreset(email)
	if err != nil {
		return YearSummary{}, err
	}

	overrides, err := s.repo.GetOverrides(year, email)
	if err != nil {
		return YearSummary{}, err
	}

	holidays := genevaHolidays(year)
	holidaySet := make(map[string]bool, len(holidays))
	for _, h := range holidays {
		holidaySet[h.Date] = true
	}

	remoteSet := make(map[int]bool, len(preset.RemoteDays))
	for _, d := range preset.RemoteDays {
		remoteSet[d] = true
	}

	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)

	var days []DaySummary
	// Totals in halves; converted to days (×0.5) at the end.
	var workedHalves, remoteHalves, leaveHalves int

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		wd := int(d.Weekday())
		isWeekend := wd == 0 || wd == 6
		isHoliday := holidaySet[ds]

		ov := overrides[ds] // zero-value Override if none

		var amState, pmState string
		if !isWeekend && !isHoliday {
			amState = effectiveHalf(ov.AM, remoteSet[wd])
			pmState = effectiveHalf(ov.PM, remoteSet[wd])
			for _, st := range []string{amState, pmState} {
				switch st {
				case "leave":
					leaveHalves++
				case "remote":
					workedHalves++
					remoteHalves++
				case "office":
					workedHalves++
				}
			}
		}

		days = append(days, DaySummary{
			Date:       ds,
			Weekday:    wd,
			IsWeekend:  isWeekend,
			IsHoliday:  isHoliday,
			AMState:    amState,
			PMState:    pmState,
			AMOverride: ov.AM,
			PMOverride: ov.PM,
		})
	}

	totalWorked := float64(workedHalves) / 2
	totalRemote := float64(remoteHalves) / 2
	totalOnSite := totalWorked - totalRemote
	totalLeave := float64(leaveHalves) / 2

	var remotePct, onSitePct float64
	if workedHalves > 0 {
		remotePct = math.Round(float64(remoteHalves)/float64(workedHalves)*10000) / 100
		onSitePct = math.Round(float64(workedHalves-remoteHalves)/float64(workedHalves)*10000) / 100
	}

	overThreshold := remotePct > 40.0

	// Halves to recover = TT halves to convert to on-site so that remote_pct ≤ 40%.
	// Solve: (remoteHalves - x) / workedHalves ≤ 0.4  →  x ≥ remoteHalves - 0.4*workedHalves
	daysToRecover := 0.0
	if overThreshold && workedHalves > 0 {
		needed := float64(remoteHalves) - 0.4*float64(workedHalves)
		halves := math.Ceil(needed)
		if halves < 0 {
			halves = 0
		}
		daysToRecover = halves / 2
	}

	return YearSummary{
		Year:          year,
		Days:          days,
		Holidays:      holidays,
		TotalWorked:   totalWorked,
		TotalRemote:   totalRemote,
		TotalOnSite:   totalOnSite,
		TotalLeave:    totalLeave,
		RemotePct:     remotePct,
		OnSitePct:     onSitePct,
		DaysToRecover: daysToRecover,
		OverThreshold: overThreshold,
	}, nil
}

// effectiveHalf resolves one half-day's state: the override if set, otherwise the
// preset (remote when presetRemote is true, else office).
func effectiveHalf(override string, presetRemote bool) string {
	if override != "" {
		return override
	}
	if presetRemote {
		return "remote"
	}
	return "office"
}
