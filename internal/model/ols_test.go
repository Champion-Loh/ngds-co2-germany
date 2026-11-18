package model

import (
	"errors"
	"math"
	"testing"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// A perfect line y = 3 + 2x must be recovered exactly with zero residuals.
func TestFitOLSExactLine(t *testing.T) {
	var X [][]float64
	var y []float64
	for i := 0; i < 10; i++ {
		x := float64(i)
		X = append(X, []float64{1, x})
		y = append(y, 3+2*x)
	}
	r, err := FitOLS(X, y)
	if err != nil {
		t.Fatal(err)
	}
	if !approx(r.Coef[0], 3, 1e-9) || !approx(r.Coef[1], 2, 1e-9) {
		t.Fatalf("got coef %v, want [3 2]", r.Coef)
	}
	if r.SSE > 1e-18 {
		t.Fatalf("SSE should be ~0, got %g", r.SSE)
	}
}

// Reference values computed independently with numpy.linalg.lstsq.
func TestFitOLSMatchesNumpy(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4, 5}
	ys := []float64{1.1, 2.9, 5.2, 6.8, 9.1, 10.9}
	X := make([][]float64, len(xs))
	for i, x := range xs {
		X[i] = []float64{1, x}
	}
	r, err := FitOLS(X, ys)
	if err != nil {
		t.Fatal(err)
	}
	// numpy: coef [1.05714286, 1.97714286], se [0.12048654, 0.03979540]
	want := []float64{1.05714286, 1.97714286}
	wantSE := []float64{0.12048654, 0.03979540}
	for i := range want {
		if !approx(r.Coef[i], want[i], 1e-6) || !approx(r.SE[i], wantSE[i], 1e-6) {
			t.Fatalf("term %d: got coef %.8f se %.8f, want %.8f %.8f", i, r.Coef[i], r.SE[i], want[i], wantSE[i])
		}
	}
}

func TestFitOLSRejectsCollinear(t *testing.T) {
	X := [][]float64{{1, 1, 2}, {1, 2, 4}, {1, 3, 6}, {1, 4, 8}} // col3 = 2*col2
	_, err := FitOLS(X, []float64{1, 2, 3, 4})
	if !errors.Is(err, ErrRankDeficient) {
		t.Fatalf("expected ErrRankDeficient, got %v", err)
	}
}

func TestFitOLSInputValidation(t *testing.T) {
	cases := map[string]struct {
		X [][]float64
		y []float64
	}{
		"length mismatch": {[][]float64{{1, 0}, {1, 1}, {1, 2}}, []float64{1, 2}},
		"too few rows":    {[][]float64{{1, 0}, {1, 1}}, []float64{1, 2}},
		"NaN in X":        {[][]float64{{1, 0}, {1, math.NaN()}, {1, 2}}, []float64{1, 2, 3}},
		"ragged row":      {[][]float64{{1, 0}, {1}, {1, 2}}, []float64{1, 2, 3}},
	}
	for name, c := range cases {
		if _, err := FitOLS(c.X, c.y); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// P(T > 2.0) with 10 df = 0.036694 (scipy.stats.t.sf).
func TestStudentT(t *testing.T) {
	if got := studentTUpper(2.0, 10); !approx(got, 0.0366940, 1e-6) {
		t.Fatalf("got %.7f", got)
	}
}

func TestEvaluate(t *testing.T) {
	m := Evaluate([]float64{100, 200, 300}, []float64{110, 190, 300})
	if !approx(m.MAE, 20.0/3, 1e-9) || !approx(m.RMSE, math.Sqrt(200.0/3), 1e-9) || m.N != 3 {
		t.Fatalf("unexpected metrics %+v", m)
	}
	if !approx(m.Bias, 0, 1e-9) {
		t.Fatalf("bias should be 0, got %g", m.Bias)
	}
}

func TestDurbinWatsonNoAutocorrelation(t *testing.T) {
	// alternating residuals -> strong negative autocorrelation -> DW near 4
	if dw := DurbinWatson([]float64{1, -1, 1, -1, 1, -1}); dw < 3 {
		t.Fatalf("expected DW > 3, got %g", dw)
	}
}
