// Package analysis compares CO2 emissions across years, decades and
// historical periods, and runs the model evaluation.
package analysis

import (
	"math"
	"sort"
	"strconv"

	"github.com/Champion-Loh/ngds-co2-germany/internal/data"
	"github.com/Champion-Loh/ngds-co2-germany/internal/model"
)

// GroupSummary describes one decade or period.
type GroupSummary struct {
	Label      string
	StartYear  int
	EndYear    int
	N          int
	Mean       float64
	Min, Max   float64
	MinYear    int
	MaxYear    int
	StartValue float64
	EndValue   float64
	ChangePct  float64 // (end - start) / start * 100
	SlopeMt    float64 // OLS slope of CO2 on year (Mt per year)
	SlopeSE    float64
	SlopeP     float64
	AnnualPct  float64 // 100*(exp(b)-1) from ln(CO2) on year: average % change per year
	R2         float64
	MeanVsBase float64 // mean % vs the base year (1990)
}

// ByDecade groups rows by decade.
func ByDecade(rows []data.Row) []GroupSummary {
	return group(rows, func(r data.Row) string { return decadeLabel(r.Decade) })
}

// ByPeriod groups rows by the historical period label.
func ByPeriod(rows []data.Row) []GroupSummary {
	return group(rows, func(r data.Row) string { return r.Period })
}

func decadeLabel(d int) string {
	return strconv.Itoa(d) + "s"
}

func group(rows []data.Row, key func(data.Row) string) []GroupSummary {
	buckets := map[string][]data.Row{}
	var order []string
	for _, r := range rows {
		k := key(r)
		if _, ok := buckets[k]; !ok {
			order = append(order, k)
		}
		buckets[k] = append(buckets[k], r)
	}
	out := make([]GroupSummary, 0, len(order))
	for _, k := range order {
		out = append(out, summarise(k, buckets[k]))
	}
	return out
}

func summarise(label string, rs []data.Row) GroupSummary {
	g := GroupSummary{Label: label, N: len(rs), StartYear: rs[0].Year, EndYear: rs[len(rs)-1].Year,
		Min: math.Inf(1), Max: math.Inf(-1), StartValue: rs[0].CO2, EndValue: rs[len(rs)-1].CO2}
	var sum, sumBase float64
	for _, r := range rs {
		sum += r.CO2
		sumBase += r.PctVsBase
		if r.CO2 < g.Min {
			g.Min, g.MinYear = r.CO2, r.Year
		}
		if r.CO2 > g.Max {
			g.Max, g.MaxYear = r.CO2, r.Year
		}
	}
	g.Mean = sum / float64(len(rs))
	g.MeanVsBase = sumBase / float64(len(rs))
	g.ChangePct = 100 * (g.EndValue - g.StartValue) / g.StartValue

	g.SlopeMt, g.SlopeSE, g.SlopeP, g.AnnualPct, g.R2 = math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()
	if len(rs) >= 3 { // need n > p = 2 for a slope with a standard error
		X := make([][]float64, len(rs))
		y := make([]float64, len(rs))
		ly := make([]float64, len(rs))
		for i, r := range rs {
			X[i] = []float64{1, float64(r.Year - rs[0].Year)}
			y[i] = r.CO2
			ly[i] = r.LogCO2
		}
		if fit, err := model.FitOLS(X, y); err == nil {
			g.SlopeMt, g.SlopeSE, g.SlopeP = fit.Coef[1], fit.SE[1], fit.PValue[1]
			g.R2 = model.Evaluate(y, fit.Fitted).R2
		}
		if fit, err := model.FitOLS(X, ly); err == nil {
			g.AnnualPct = 100 * (math.Exp(fit.Coef[1]) - 1)
		}
	}
	return g
}

// Extremes returns the k largest falls and k largest rises in YoY % change.
func Extremes(rows []data.Row, k int) (falls, rises []data.Row) {
	var rs []data.Row
	for _, r := range rows {
		if !math.IsNaN(r.YoYPct) {
			rs = append(rs, r)
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].YoYPct < rs[j].YoYPct })
	if k > len(rs) {
		k = len(rs)
	}
	falls = append(falls, rs[:k]...)
	for i := len(rs) - 1; i >= len(rs)-k; i-- {
		rises = append(rises, rs[i])
	}
	return falls, rises
}
