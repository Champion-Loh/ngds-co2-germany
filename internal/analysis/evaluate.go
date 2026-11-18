package analysis

import (
	"fmt"
	"math"

	"github.com/Champion-Loh/ngds-co2-germany/internal/data"
	"github.com/Champion-Loh/ngds-co2-germany/internal/model"
)

// ModelEval collects everything reported for one model.
type ModelEval struct {
	Spec     model.Spec
	TrainFit *model.Fitted // fitted on the training years only
	FullFit  *model.Fitted // refitted on all years (for coefficients + forecast)
	Train    model.Metrics // in-sample
	AdjR2    float64
	DW       float64
	Test     model.Metrics // hold-out, recursive multi-step forecast
	TestPred []float64     // aligned with the test rows
	CV       model.Metrics // rolling-origin, one-step-ahead
	CVPred   map[int]float64
	Forecast []float64 // aligned with future rows
}

// Config controls the evaluation.
type Config struct {
	TestYears  int // size of the chronological hold-out at the end of the series
	CVYears    int // number of rolling one-step-ahead origins
	ForecastTo int // last year to project
	BreakYear  int
}

// Split divides rows chronologically. No shuffling: with time series a
// random split would let the model "see the future".
func Split(rows []data.Row, testYears int) (train, test []data.Row, err error) {
	if testYears <= 0 || testYears >= len(rows)-5 {
		return nil, nil, fmt.Errorf("test-years must be between 1 and %d, got %d", len(rows)-6, testYears)
	}
	cut := len(rows) - testYears
	return rows[:cut], rows[cut:], nil
}

// EvaluateAll fits every spec, scores it on the hold-out and on a rolling
// backtest, refits on all data and forecasts to cfg.ForecastTo.
func EvaluateAll(rows []data.Row, specs []model.Spec, cfg Config) ([]ModelEval, []data.Row, error) {
	train, test, err := Split(rows, cfg.TestYears)
	if err != nil {
		return nil, nil, err
	}
	future := FutureRows(rows[len(rows)-1], cfg.ForecastTo, cfg.BreakYear)

	var out []ModelEval
	for _, s := range specs {
		ev := ModelEval{Spec: s, CVPred: map[int]float64{}}

		// 1. train fit + in-sample metrics
		ev.TrainFit, err = model.Fit(s, train)
		if err != nil {
			return nil, nil, err
		}
		ev.Train = model.Evaluate(ev.TrainFit.Actual, ev.TrainFit.Pred)
		ev.AdjR2 = math.NaN()
		ev.DW = math.NaN()
		if ev.TrainFit.OLS != nil {
			ev.AdjR2 = model.AdjR2(ev.Train.R2, ev.Train.N, ev.TrainFit.OLS.P)
			ev.DW = model.DurbinWatson(ev.TrainFit.OLS.Residuals)
		}

		// 2. hold-out: forecast the whole test window from the end of training
		ev.TestPred = ev.TrainFit.Forecast(test, train[len(train)-1].CO2)
		ev.Test = model.Evaluate(co2Of(test), ev.TestPred)

		// 3. rolling-origin backtest: for each of the last CVYears years,
		// fit on all earlier years and predict just that year.
		var cvAct, cvPred []float64
		for i := len(rows) - cfg.CVYears; i < len(rows); i++ {
			f, err := model.Fit(s, rows[:i])
			if err != nil {
				return nil, nil, fmt.Errorf("backtest %d: %w", rows[i].Year, err)
			}
			p := f.OneStep(rows[i])
			ev.CVPred[rows[i].Year] = p
			cvAct = append(cvAct, rows[i].CO2)
			cvPred = append(cvPred, p)
		}
		ev.CV = model.Evaluate(cvAct, cvPred)

		// 4. refit on everything and project forward
		ev.FullFit, err = model.Fit(s, rows)
		if err != nil {
			return nil, nil, err
		}
		ev.Forecast = ev.FullFit.Forecast(future, rows[len(rows)-1].CO2)

		out = append(out, ev)
	}
	return out, future, nil
}

// FutureRows builds feature rows for the years after the data ends.
func FutureRows(last data.Row, toYear, breakYear int) []data.Row {
	var out []data.Row
	for y := last.Year + 1; y <= toYear; y++ {
		r := data.Row{Year: y, T: last.T + float64(y-last.Year), Lag1: math.NaN(),
			Decade: y / 10 * 10, Period: data.PeriodFor(y)}
		if y >= breakYear {
			r.PostBreak = 1
		}
		out = append(out, r)
	}
	return out
}

func co2Of(rows []data.Row) []float64 {
	v := make([]float64, len(rows))
	for i, r := range rows {
		v[i] = r.CO2
	}
	return v
}
