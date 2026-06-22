package portfolio

import (
	"math"
	"time"
)

// MonthlyPayment computes the fixed monthly payment for a loan using the
// standard amortization formula: M = P * r / (1 - (1+r)^-n)
// where r = monthly rate = TAEG/12/100, n = duration in months.
func MonthlyPayment(borrowed float64, taegPct float64, durationMonths int) float64 {
	r := taegPct / 12.0 / 100.0
	if r == 0 {
		return borrowed / float64(durationMonths)
	}
	n := float64(durationMonths)
	return borrowed * r / (1 - math.Pow(1+r, -n))
}

// RemainingCapital returns the outstanding principal as of asOf.
// Repayments are assumed to occur on the 1st of each month starting the month
// after startDate. Returns 0 if the loan is fully repaid.
func RemainingCapital(startDate time.Time, durationMonths int, taegPct float64, borrowed float64, asOf time.Time) float64 {
	r := taegPct / 12.0 / 100.0
	m := MonthlyPayment(borrowed, taegPct, durationMonths)

	// Count how many payments have been made: payments happen on the 1st of each
	// month, starting the 1st of the month after startDate.
	firstPayment := time.Date(startDate.Year(), startDate.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	if asOf.Before(firstPayment) {
		return borrowed
	}

	// Months elapsed since first payment (inclusive if asOf >= 1st of that month).
	months := (asOf.Year()-firstPayment.Year())*12 + int(asOf.Month()-firstPayment.Month()) + 1
	if months >= durationMonths {
		return 0
	}

	// Standard remaining balance formula: P*(1+r)^k - M*((1+r)^k - 1)/r
	k := float64(months)
	if r == 0 {
		return math.Max(0, borrowed-m*k)
	}
	remaining := borrowed*math.Pow(1+r, k) - m*(math.Pow(1+r, k)-1)/r
	return math.Max(0, math.Round(remaining*100)/100)
}

// --- Repo methods ---

// GetAllDettes returns the dette parameters for all dette assets belonging to the given user.
func (r *Repo) GetAllDettes(email string) (map[int]DetteInfo, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT da.asset_id, da.start_date, da.duration_months, da.taeg, da.amount_borrowed
		FROM dette_assets da
		JOIN assets a ON a.id = da.asset_id
		WHERE a.user_email = $1
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]DetteInfo)
	for rows.Next() {
		var d DetteInfo
		var startDate time.Time
		if err := rows.Scan(&d.AssetID, &startDate, &d.DurationMonths, &d.TAEG, &d.AmountBorrowed); err != nil {
			return nil, err
		}
		d.StartDate = startDate.Format("2006-01-02")
		now := time.Now().UTC()
		d.MonthlyPayment = MonthlyPayment(d.AmountBorrowed, d.TAEG, d.DurationMonths)
		d.RemainingCapital = RemainingCapital(startDate, d.DurationMonths, d.TAEG, d.AmountBorrowed, now)
		result[d.AssetID] = d
	}
	return result, nil
}

// UpsertDette inserts or updates the dette parameters for an asset.
func (r *Repo) UpsertDette(d DetteInfo) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO dette_assets (asset_id, start_date, duration_months, taeg, amount_borrowed)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (asset_id) DO UPDATE
			SET start_date = $2, duration_months = $3, taeg = $4, amount_borrowed = $5
	`, d.AssetID, d.StartDate, d.DurationMonths, d.TAEG, d.AmountBorrowed)
	return err
}

// DeleteDette removes the dette parameters for an asset (called when asset is deleted).
func (r *Repo) DeleteDette(assetID int) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM dette_assets WHERE asset_id = $1`, assetID)
	return err
}
