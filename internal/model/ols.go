// Package model implements ordinary least squares regression and the
// evaluation metrics used to compare the CO2 trend models.
//
// gonum is the usual Go library for this, but it could not be downloaded in
// the build environment, so OLS is written with the standard library only.
// We use a Householder QR decomposition instead of the textbook
// beta = (X'X)^-1 X'y because forming X'X squares the condition number:
// with a quadratic trend (t and t^2) that loses precision for no benefit.
package model

import (
	"errors"
	"fmt"
	"math"
)

// OLSResult holds a fitted linear regression.
type OLSResult struct {
	Coef      []float64 // estimated coefficients, same order as the columns of X
	SE        []float64 // standard errors
	TStat     []float64 // Coef / SE
	PValue    []float64 // two-sided p-values from the t distribution
	Fitted    []float64 // X * Coef
	Residuals []float64 // y - Fitted
	N, P      int       // observations, parameters (incl. intercept)
	SSE       float64   // sum of squared residuals
	Sigma2    float64   // residual variance SSE / (N - P)
}

// ErrRankDeficient is returned when columns of X are (nearly) collinear.
var ErrRankDeficient = errors.New("design matrix is rank deficient (collinear predictors)")

// FitOLS fits y = X*beta + e by least squares. X is row-major: X[i] is the
// feature vector for observation i and must include an intercept column if
// one is wanted.
func FitOLS(X [][]float64, y []float64) (*OLSResult, error) {
	n := len(X)
	if n == 0 || n != len(y) {
		return nil, fmt.Errorf("X has %d rows but y has %d values", n, len(y))
	}
	p := len(X[0])
	if p == 0 {
		return nil, errors.New("X has no columns")
	}
	if n <= p {
		return nil, fmt.Errorf("need more observations (%d) than parameters (%d)", n, p)
	}
	for i, row := range X {
		if len(row) != p {
			return nil, fmt.Errorf("row %d has %d columns, expected %d", i, len(row), p)
		}
		for _, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("row %d contains NaN/Inf", i)
			}
		}
		if math.IsNaN(y[i]) || math.IsInf(y[i], 0) {
			return nil, fmt.Errorf("y[%d] is NaN/Inf", i)
		}
	}

	// Copy so we never modify the caller's data.
	A := make([][]float64, n)
	for i := range X {
		A[i] = append([]float64(nil), X[i]...)
	}
	b := append([]float64(nil), y...)

	// Householder QR: for each column k, reflect rows k..n-1 so that
	// A[k+1:, k] becomes zero. Apply the same reflection to b, giving Q'y.
	maxDiag := 0.0
	for k := 0; k < p; k++ {
		norm := 0.0
		for i := k; i < n; i++ {
			norm += A[i][k] * A[i][k]
		}
		norm = math.Sqrt(norm)
		if norm == 0 {
			return nil, ErrRankDeficient
		}
		alpha := -norm
		if A[k][k] < 0 {
			alpha = norm
		}
		v := make([]float64, n)
		v[k] = A[k][k] - alpha
		for i := k + 1; i < n; i++ {
			v[i] = A[i][k]
		}
		vv := 0.0
		for i := k; i < n; i++ {
			vv += v[i] * v[i]
		}
		if vv == 0 {
			continue // column already in the right form
		}
		for j := k; j < p; j++ {
			dot := 0.0
			for i := k; i < n; i++ {
				dot += v[i] * A[i][j]
			}
			f := 2 * dot / vv
			for i := k; i < n; i++ {
				A[i][j] -= f * v[i]
			}
		}
		dot := 0.0
		for i := k; i < n; i++ {
			dot += v[i] * b[i]
		}
		f := 2 * dot / vv
		for i := k; i < n; i++ {
			b[i] -= f * v[i]
		}
		maxDiag = math.Max(maxDiag, math.Abs(A[k][k]))
	}

	// Rank check: a tiny diagonal entry in R means a near-collinear column.
	for k := 0; k < p; k++ {
		if math.Abs(A[k][k]) < 1e-10*maxDiag {
			return nil, ErrRankDeficient
		}
	}

	// Back-substitution: R * beta = (Q'y)[0:p]
	beta := make([]float64, p)
	for k := p - 1; k >= 0; k-- {
		s := b[k]
		for j := k + 1; j < p; j++ {
			s -= A[k][j] * beta[j]
		}
		beta[k] = s / A[k][k]
	}

	res := &OLSResult{Coef: beta, N: n, P: p}
	res.Fitted = make([]float64, n)
	res.Residuals = make([]float64, n)
	for i := 0; i < n; i++ {
		res.Fitted[i] = dot(X[i], beta)
		res.Residuals[i] = y[i] - res.Fitted[i]
		res.SSE += res.Residuals[i] * res.Residuals[i]
	}
	df := n - p
	res.Sigma2 = res.SSE / float64(df)

	// Var(beta) = sigma^2 (X'X)^-1 = sigma^2 R^-1 R^-T
	Rinv := invertUpper(A, p)
	res.SE = make([]float64, p)
	res.TStat = make([]float64, p)
	res.PValue = make([]float64, p)
	for k := 0; k < p; k++ {
		v := 0.0
		for j := k; j < p; j++ {
			v += Rinv[k][j] * Rinv[k][j]
		}
		res.SE[k] = math.Sqrt(res.Sigma2 * v)
		if res.SE[k] > 0 {
			res.TStat[k] = beta[k] / res.SE[k]
			res.PValue[k] = 2 * studentTUpper(math.Abs(res.TStat[k]), float64(df))
		} else {
			res.TStat[k] = math.NaN()
			res.PValue[k] = math.NaN()
		}
	}
	return res, nil
}

// Predict returns x . coef.
func Predict(x, coef []float64) float64 { return dot(x, coef) }

func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// invertUpper inverts the p x p upper-triangular block stored in R[0:p][0:p].
func invertUpper(R [][]float64, p int) [][]float64 {
	inv := make([][]float64, p)
	for i := range inv {
		inv[i] = make([]float64, p)
	}
	for j := 0; j < p; j++ {
		inv[j][j] = 1 / R[j][j]
		for i := j - 1; i >= 0; i-- {
			s := 0.0
			for k := i + 1; k <= j; k++ {
				s += R[i][k] * inv[k][j]
			}
			inv[i][j] = -s / R[i][i]
		}
	}
	return inv
}

// studentTUpper returns P(T > t) for a t distribution with df degrees of
// freedom, using the regularised incomplete beta function.
func studentTUpper(t, df float64) float64 {
	x := df / (df + t*t)
	return 0.5 * regIncBeta(df/2, 0.5, x)
}

// regIncBeta computes I_x(a, b) with the continued-fraction method
// (Numerical Recipes, section 6.4).
func regIncBeta(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lga, _ := math.Lgamma(a)
	lgb, _ := math.Lgamma(b)
	lgab, _ := math.Lgamma(a + b)
	front := math.Exp(lgab - lga - lgb + a*math.Log(x) + b*math.Log(1-x))
	if x < (a+1)/(a+b+2) {
		return front * betaCF(a, b, x) / a
	}
	return 1 - front*betaCF(b, a, 1-x)/b
}

func betaCF(a, b, x float64) float64 {
	const maxIter, eps, tiny = 200, 3e-14, 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c, d := 1.0, 1-qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= maxIter; m++ {
		fm := float64(m)
		aa := fm * (b - fm) * x / ((qam + 2*fm) * (a + 2*fm))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + 2*fm) * (qap + 2*fm))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < eps {
			break
		}
	}
	return h
}
