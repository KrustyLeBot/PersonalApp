package walk

import (
	"math"
	"time"
)

// Service computes walking summaries.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// ComputeYear builds the full year grid plus the current week/month totals.
func (s *Service) ComputeYear(year int, email string) (YearSummary, error) {
	entries, err := s.repo.GetYear(year, email)
	if err != nil {
		return YearSummary{}, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)

	var days []Day
	monthTotals := make([]float64, 12)
	var yearTotal float64
	var activeDays int
	best := Entry{}

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		km := entries[ds]
		if km > 0 {
			yearTotal += km
			monthTotals[int(d.Month())-1] += km
			activeDays++
			if km > best.Km {
				best = Entry{Date: ds, Km: km}
			}
		}
		days = append(days, Day{
			Date:     ds,
			Weekday:  int(d.Weekday()),
			Km:       km,
			IsFuture: d.After(today),
		})
	}

	// Week and month totals always describe the current calendar week/month,
	// which may straddle a year boundary — query them by range.
	weekStart := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7)) // Monday
	weekTotal, err := s.repo.SumRange(weekStart.Format("2006-01-02"), weekStart.AddDate(0, 0, 6).Format("2006-01-02"), email)
	if err != nil {
		return YearSummary{}, err
	}
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthTotal, err := s.repo.SumRange(monthStart.Format("2006-01-02"), monthStart.AddDate(0, 1, -1).Format("2006-01-02"), email)
	if err != nil {
		return YearSummary{}, err
	}

	dailyAvg := 0.0
	if activeDays > 0 {
		dailyAvg = round2(yearTotal / float64(activeDays))
	}
	for i := range monthTotals {
		monthTotals[i] = round2(monthTotals[i])
	}

	return YearSummary{
		Year:        year,
		Days:        days,
		WeekTotal:   round2(weekTotal),
		MonthTotal:  round2(monthTotal),
		YearTotal:   round2(yearTotal),
		MonthTotals: monthTotals,
		ActiveDays:  activeDays,
		DailyAvg:    dailyAvg,
		BestDay:     best,
	}, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
