package model

import "math"

// Metrics are the evaluation scores reported for each model.
type Metrics struct {
	N    int
	RMSE float64 // root mean squared error (Mt) - penalises big misses
	MAE  float64 // mean absolute error (Mt) - "typical miss"
	MAPE float64 // mean absolute percentage error (%)
	R2   float64 // share of variance explained (can be negative out of sample)
	Bias float64 // mean of (predicted - actual); > 0 means over-predicting
}

// Evaluate compares predictions with actual values. Pairs where either value
// is NaN are skipped.
func Evaluate(actual, pred []float64) Metrics {
	var m Metrics
	var sse, sae, sape, sbias, sum float64
	var a []float64
	for i := range actual {
		if math.IsNaN(actual[i]) || math.IsNaN(pred[i]) {
			continue
		}
		e := pred[i] - actual[i]
		sse += e * e
		sae += math.Abs(e)
		if actual[i] != 0 {
			sape += math.Abs(e / actual[i])
		}
		sbias += e
		sum += actual[i]
		a = append(a, actual[i])
		m.N++
	}
	if m.N == 0 {
		return Metrics{RMSE: math.NaN(), MAE: math.NaN(), MAPE: math.NaN(), R2: math.NaN(), Bias: math.NaN()}
	}
	n := float64(m.N)
	m.RMSE = math.Sqrt(sse / n)
	m.MAE = sae / n
	m.MAPE = 100 * sape / n
	m.Bias = sbias / n
	mean := sum / n
	var sst float64
	for _, v := range a {
		sst += (v - mean) * (v - mean)
	}
	if sst > 0 {
		m.R2 = 1 - sse/sst
	} else {
		m.R2 = math.NaN()
	}
	return m
}

// AdjR2 adjusts R^2 for the number of parameters p (incl. intercept).
func AdjR2(r2 float64, n, p int) float64 {
	if n-p <= 0 {
		return math.NaN()
	}
	return 1 - (1-r2)*float64(n-1)/float64(n-p)
}

// AIC (Gaussian, up to a constant) = n*ln(SSE/n) + 2p. Lower is better.
// Only comparable between models fitted to the same target and rows.
func AIC(sse float64, n, p int) float64 {
	return float64(n)*math.Log(sse/float64(n)) + 2*float64(p)
}

// DurbinWatson tests residuals for first-order autocorrelation.
// About 2 = no autocorrelation; well below 2 = positive autocorrelation,
// which makes OLS standard errors (and p-values) too optimistic.
func DurbinWatson(resid []float64) float64 {
	var num, den float64
	for i, e := range resid {
		den += e * e
		if i > 0 {
			d := e - resid[i-1]
			num += d * d
		}
	}
	if den == 0 {
		return math.NaN()
	}
	return num / den
}
