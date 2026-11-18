package model

import (
	"fmt"
	"math"

	"github.com/Champion-Loh/ngds-co2-germany/internal/data"
)

// Spec describes one candidate regression model.
type Spec struct {
	Name     string
	Formula  string
	Terms    []string                   // coefficient names, same order as Features
	Features func(r data.Row) []float64 // design-matrix row (includes intercept)
	Target   func(r data.Row) float64   // what the regression is fitted on
	Inverse  func(v float64) float64    // maps a prediction back to Mt CO2
	MinYear  int                        // only rows with Year >= MinYear are used for fitting
	UsesLag  bool                       // true if Features reads Lag1 (forecast recursively)
	Fixed    []float64                  // if set, coefficients are fixed (no fitting), e.g. naive model
}

func identity(v float64) float64 { return v }
func co2(r data.Row) float64     { return r.CO2 }

// Specs returns the candidate models.
//
// firstYear is the first year of the data (t = 0) and breakYear the start of
// the post-reunification regime used by the piecewise model.
func Specs(firstYear, breakYear int) []Spec {
	tb := float64(breakYear - firstYear) // t value of the break year
	return []Spec{
		{
			Name: "Naive (random walk)", Formula: "CO2_t = CO2_{t-1}",
			Terms: []string{"lag1"}, Features: func(r data.Row) []float64 { return []float64{r.Lag1} },
			Target: co2, Inverse: identity, UsesLag: true, Fixed: []float64{1},
		},
		{
			Name: "Linear trend", Formula: "CO2 = b0 + b1*t",
			Terms: []string{"intercept", "t"}, Features: func(r data.Row) []float64 { return []float64{1, r.T} },
			Target: co2, Inverse: identity,
		},
		{
			Name: "Quadratic trend", Formula: "CO2 = b0 + b1*t + b2*t^2",
			Terms:    []string{"intercept", "t", "t2"},
			Features: func(r data.Row) []float64 { return []float64{1, r.T, r.T * r.T} },
			Target:   co2, Inverse: identity,
		},
		{
			Name:    fmt.Sprintf("Piecewise (break %d)", breakYear),
			Formula: fmt.Sprintf("CO2 = b0 + b1*t + b2*post + b3*post*(t-%g)", tb),
			Terms:   []string{"intercept", "t", "post_level_shift", "post_slope_change"},
			Features: func(r data.Row) []float64 {
				p := float64(r.PostBreak)
				return []float64{1, r.T, p, p * (r.T - tb)}
			},
			Target: co2, Inverse: identity,
		},
		{
			Name: fmt.Sprintf("Log-linear (%d+)", breakYear), Formula: "ln(CO2) = b0 + b1*t",
			Terms: []string{"intercept", "t"}, Features: func(r data.Row) []float64 { return []float64{1, r.T} },
			Target: func(r data.Row) float64 { return r.LogCO2 }, Inverse: math.Exp, MinYear: breakYear,
		},
		{
			Name: "AR(1) + trend", Formula: "CO2_t = b0 + b1*CO2_{t-1} + b2*t",
			Terms:    []string{"intercept", "lag1", "t"},
			Features: func(r data.Row) []float64 { return []float64{1, r.Lag1, r.T} },
			Target:   co2, Inverse: identity, UsesLag: true,
		},
	}
}

// Fitted is a Spec after fitting.
type Fitted struct {
	Spec   Spec
	OLS    *OLSResult // nil for fixed-coefficient models
	Coef   []float64
	Years  []int     // years used in fitting
	Actual []float64 // actual CO2 for those years (Mt)
	Pred   []float64 // in-sample fitted values back on the Mt scale
}

// usable reports whether a row has finite features and target.
func usable(s Spec, r data.Row) bool {
	if r.Year < s.MinYear {
		return false
	}
	for _, v := range s.Features(r) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	t := s.Target(r)
	return !math.IsNaN(t) && !math.IsInf(t, 0)
}

// Fit estimates the model on the given rows.
func Fit(s Spec, rows []data.Row) (*Fitted, error) {
	var X [][]float64
	var y []float64
	f := &Fitted{Spec: s}
	for _, r := range rows {
		if !usable(s, r) {
			continue
		}
		X = append(X, s.Features(r))
		y = append(y, s.Target(r))
		f.Years = append(f.Years, r.Year)
		f.Actual = append(f.Actual, r.CO2)
	}
	if len(X) == 0 {
		return nil, fmt.Errorf("%s: no usable rows", s.Name)
	}
	if s.Fixed != nil {
		f.Coef = s.Fixed
	} else {
		ols, err := FitOLS(X, y)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name, err)
		}
		f.OLS = ols
		f.Coef = ols.Coef
	}
	for _, x := range X {
		f.Pred = append(f.Pred, s.Inverse(Predict(x, f.Coef)))
	}
	return f, nil
}

// Forecast predicts CO2 for future rows. For models that use the lag, the
// previous *prediction* replaces the unknown actual value (recursive
// multi-step forecast), which is the honest way to evaluate them on a
// hold-out period. lastActual is the last observed CO2 before rows[0].
func (f *Fitted) Forecast(rows []data.Row, lastActual float64) []float64 {
	out := make([]float64, len(rows))
	prev := lastActual
	for i, r := range rows {
		rr := r
		if f.Spec.UsesLag {
			rr.Lag1 = prev
		}
		out[i] = f.Spec.Inverse(Predict(f.Spec.Features(rr), f.Coef))
		prev = out[i]
	}
	return out
}

// OneStep predicts each row using its real previous-year value (for lag
// models) - i.e. "next year only" forecasts.
func (f *Fitted) OneStep(r data.Row) float64 {
	return f.Spec.Inverse(Predict(f.Spec.Features(r), f.Coef))
}
