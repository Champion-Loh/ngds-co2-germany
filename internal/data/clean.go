package data

import (
	"fmt"
	"math"
	"sort"
)

// Row is one year of the cleaned, feature-engineered table used for modelling.
type Row struct {
	Year       int
	T          float64 // years since the first year (1970 -> 0); used as the trend variable
	CO2        float64 // Mt CO2e
	LogCO2     float64 // natural log of CO2, for the log-linear (constant % change) model
	Lag1       float64 // CO2 in the previous year; NaN for the first year
	YoYChange  float64 // CO2 - Lag1 (Mt); NaN for the first year
	YoYPct     float64 // 100 * YoYChange / Lag1; NaN for the first year
	PctVsBase  float64 // % change versus the base year (1990 by default)
	Decade     int     // e.g. 1990
	Period     string  // policy/historical period label, see PeriodFor
	PostBreak  int     // 1 if Year >= break year, else 0
	Imputed    bool    // true if CO2 was missing/invalid and filled by interpolation
	Duplicate  bool    // true if the year appeared more than once in the raw file
	OutlierYoY bool    // true if the YoY % change is unusual (robust z-score > threshold)
}

// CleanOptions controls the cleaning step.
type CleanOptions struct {
	BaseYear         int     // reference year for "% vs base" (Kyoto/German targets use 1990)
	BreakYear        int     // first year of the post-break regime (1990 = reunification)
	OutlierThreshold float64 // robust z-score cut-off for flagging YoY changes (3.5 is standard)
}

// DefaultCleanOptions returns the settings used in the report.
func DefaultCleanOptions() CleanOptions {
	return CleanOptions{BaseYear: 1990, BreakYear: 1990, OutlierThreshold: 3.5}
}

// CleanReport summarises what the cleaning step did, for the data-quality log.
type CleanReport struct {
	RowsIn         int
	DuplicateYears []int
	GapYearsFilled []int
	ImputedYears   []int
	OutlierYears   []int
	FirstYear      int
	LastYear       int
	RowsOut        int
}

// Clean turns raw records into a complete, gap-free yearly table.
//
// Steps:
//  1. Remove duplicate years (keep the first valid value, flag the year).
//  2. Insert any missing years between the first and last year.
//  3. Fill missing/invalid CO2 values by linear interpolation between the
//     nearest valid neighbours (nearest value at the edges). We interpolate
//     rather than drop because the models need consecutive years for the lag
//     variable and because emissions change smoothly from year to year.
//  4. Build features (time index, log, lag, YoY change, % vs base, labels).
//  5. Flag (do not remove) unusually large YoY changes. Big drops such as
//     2009 (financial crisis) and 2020 (COVID) are real events, so removing
//     them would hide exactly what we want to explain.
func Clean(recs []RawRecord, opt CleanOptions) ([]Row, CleanReport, error) {
	rep := CleanReport{RowsIn: len(recs)}
	if len(recs) == 0 {
		return nil, rep, fmt.Errorf("no records to clean")
	}

	// 1. de-duplicate
	byYear := map[int]float64{}
	dup := map[int]bool{}
	for _, r := range recs {
		if prev, seen := byYear[r.Year]; seen {
			dup[r.Year] = true
			if math.IsNaN(prev) && !math.IsNaN(r.Value) {
				byYear[r.Year] = r.Value // first copy was invalid; take the valid one
			}
			continue
		}
		byYear[r.Year] = r.Value
	}
	for y := range dup {
		rep.DuplicateYears = append(rep.DuplicateYears, y)
	}
	sort.Ints(rep.DuplicateYears)

	// 2. complete the year sequence
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Ints(years)
	first, last := years[0], years[len(years)-1]
	rep.FirstYear, rep.LastYear = first, last

	n := last - first + 1
	vals := make([]float64, n)
	imputed := make([]bool, n)
	for i := 0; i < n; i++ {
		y := first + i
		v, ok := byYear[y]
		if !ok {
			rep.GapYearsFilled = append(rep.GapYearsFilled, y)
			v = math.NaN()
		}
		vals[i] = v
	}

	// 3. interpolate missing values
	validCount := 0
	for _, v := range vals {
		if !math.IsNaN(v) {
			validCount++
		}
	}
	if validCount < 2 {
		return nil, rep, fmt.Errorf("need at least 2 valid CO2 values, found %d", validCount)
	}
	for i := range vals {
		if !math.IsNaN(vals[i]) {
			continue
		}
		imputed[i] = true
		rep.ImputedYears = append(rep.ImputedYears, first+i)
		lo, hi := i-1, i+1
		for lo >= 0 && math.IsNaN(vals[lo]) {
			lo--
		}
		for hi < n && math.IsNaN(vals[hi]) {
			hi++
		}
		switch {
		case lo < 0:
			vals[i] = vals[hi]
		case hi >= n:
			vals[i] = vals[lo]
		default:
			w := float64(i-lo) / float64(hi-lo)
			vals[i] = vals[lo] + w*(vals[hi]-vals[lo])
		}
	}

	// 4. features
	baseIdx := opt.BaseYear - first
	if baseIdx < 0 || baseIdx >= n {
		return nil, rep, fmt.Errorf("base year %d is outside the data range %d-%d", opt.BaseYear, first, last)
	}
	base := vals[baseIdx]

	rows := make([]Row, n)
	for i := 0; i < n; i++ {
		y := first + i
		r := Row{
			Year:      y,
			T:         float64(i),
			CO2:       vals[i],
			LogCO2:    math.Log(vals[i]),
			Lag1:      math.NaN(),
			YoYChange: math.NaN(),
			YoYPct:    math.NaN(),
			PctVsBase: 100 * (vals[i] - base) / base,
			Decade:    y / 10 * 10,
			Period:    PeriodFor(y),
			Imputed:   imputed[i],
			Duplicate: dup[y],
		}
		if y >= opt.BreakYear {
			r.PostBreak = 1
		}
		if i > 0 {
			r.Lag1 = vals[i-1]
			r.YoYChange = vals[i] - vals[i-1]
			r.YoYPct = 100 * r.YoYChange / vals[i-1]
		}
		rows[i] = r
	}

	// 5. robust outlier flag on YoY % change (median / MAD z-score)
	var pct []float64
	for _, r := range rows[1:] {
		pct = append(pct, r.YoYPct)
	}
	med := median(pct)
	absDev := make([]float64, len(pct))
	for i, p := range pct {
		absDev[i] = math.Abs(p - med)
	}
	mad := median(absDev)
	if mad > 0 {
		for i := 1; i < n; i++ {
			z := 0.6745 * (rows[i].YoYPct - med) / mad
			if math.Abs(z) > opt.OutlierThreshold {
				rows[i].OutlierYoY = true
				rep.OutlierYears = append(rep.OutlierYears, rows[i].Year)
			}
		}
	}

	rep.RowsOut = n
	return rows, rep, nil
}

// PeriodFor labels each year with a historical/policy period for Germany.
// These boundaries are analyst choices (documented in the README), not data.
func PeriodFor(year int) string {
	switch {
	case year < 1990:
		return "1970-1989 Pre-reunification"
	case year < 2000:
		return "1990-1999 Reunification restructuring"
	case year < 2020:
		return "2000-2019 Energiewende era"
	default:
		return "2020-2022 COVID and energy crisis"
	}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return (s[m-1] + s[m]) / 2
}
