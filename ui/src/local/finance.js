export function monthlyPayment(borrowed, taegPct, durationMonths) {
  const r = taegPct / 12 / 100;
  if (r === 0) return borrowed / durationMonths;
  return (borrowed * r) / (1 - Math.pow(1 + r, -durationMonths));
}

// Go-style rounding (half away from zero), used so amounts match the old backend.
export function round2(v) {
  return (Math.sign(v) * Math.round(Math.abs(v) * 100)) / 100;
}

export function parseDate(ymd) {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d));
}

// Outstanding principal as of asOf. Repayments happen on the 1st of each month,
// starting the month after startDate. Returns 0 once fully repaid.
export function remainingCapital(startDate, durationMonths, taegPct, borrowed, asOf) {
  const r = taegPct / 12 / 100;
  const m = monthlyPayment(borrowed, taegPct, durationMonths);

  const firstPayment = new Date(Date.UTC(startDate.getUTCFullYear(), startDate.getUTCMonth() + 1, 1));
  if (asOf < firstPayment) return borrowed;

  const months =
    (asOf.getUTCFullYear() - firstPayment.getUTCFullYear()) * 12 +
    (asOf.getUTCMonth() - firstPayment.getUTCMonth()) + 1;
  if (months >= durationMonths) return 0;

  if (r === 0) return Math.max(0, borrowed - m * months);
  const remaining = borrowed * Math.pow(1 + r, months) - (m * (Math.pow(1 + r, months) - 1)) / r;
  return Math.max(0, round2(remaining));
}

export function detteInfo(assetId, d, now = new Date()) {
  const start = parseDate(d.start_date);
  return {
    asset_id: assetId,
    start_date: d.start_date,
    duration_months: d.duration_months,
    taeg: d.taeg,
    amount_borrowed: d.amount_borrowed,
    monthly_payment: monthlyPayment(d.amount_borrowed, d.taeg, d.duration_months),
    remaining_capital: remainingCapital(start, d.duration_months, d.taeg, d.amount_borrowed, now),
  };
}
