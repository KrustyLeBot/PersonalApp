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
	totalWorked := 0
	totalRemote := 0

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		wd := int(d.Weekday())
		isWeekend := wd == 0 || wd == 6
		isHoliday := holidaySet[ds]

		overrideType := overrides[ds] // "" if no override

		// Holidays and weekends can never be worked, regardless of override.
		isLeave := !isWeekend && !isHoliday && overrideType == "leave"
		isWorked := !isWeekend && !isHoliday && !isLeave

		var isRemote bool
		if isWorked {
			totalWorked++
			if overrideType == "remote" {
				isRemote = true
			} else if overrideType == "office" {
				isRemote = false
			} else {
				isRemote = remoteSet[wd]
			}
			if isRemote {
				totalRemote++
			}
		}

		days = append(days, DaySummary{
			Date:         ds,
			Weekday:      wd,
			IsWeekend:    isWeekend,
			IsHoliday:    isHoliday,
			IsLeave:      isLeave,
			IsRemote:     isRemote,
			OverrideType: overrideType,
		})
	}

	totalOnSite := totalWorked - totalRemote

	var remotePct, onSitePct float64
	if totalWorked > 0 {
		remotePct = math.Round(float64(totalRemote)/float64(totalWorked)*10000) / 100
		onSitePct = math.Round(float64(totalOnSite)/float64(totalWorked)*10000) / 100
	}

	overThreshold := remotePct > 40.0

	// Days to recover = TT days to convert to on-site so that remote_pct ≤ 40%.
	// Converting a TT day to on-site decreases totalRemote by 1; totalWorked is unchanged.
	// Solve: (totalRemote - x) / totalWorked ≤ 0.4  →  x ≥ totalRemote - 0.4*totalWorked
	daysToRecover := 0
	if overThreshold && totalWorked > 0 {
		needed := float64(totalRemote) - 0.4*float64(totalWorked)
		daysToRecover = int(math.Ceil(needed))
		if daysToRecover < 0 {
			daysToRecover = 0
		}
	}

	return YearSummary{
		Year:          year,
		Days:          days,
		Holidays:      holidays,
		TotalWorked:   totalWorked,
		TotalRemote:   totalRemote,
		TotalOnSite:   totalOnSite,
		RemotePct:     remotePct,
		OnSitePct:     onSitePct,
		DaysToRecover: daysToRecover,
		OverThreshold: overThreshold,
	}, nil
}
