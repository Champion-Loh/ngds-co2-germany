// Command co2pipeline runs the full NGDS Assignment 2 pipeline:
// load -> validate -> clean -> feature engineering -> regression models ->
// evaluation -> period comparison -> forecast, writing CSV outputs.
//
// Usage:
//
//	go run ./cmd/co2pipeline -input data/raw/NGDS_CO2_Germany.csv
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Champion-Loh/ngds-co2-germany/internal/analysis"
	"github.com/Champion-Loh/ngds-co2-germany/internal/data"
	"github.com/Champion-Loh/ngds-co2-germany/internal/model"
)

func main() {
	input := flag.String("input", "data/raw/NGDS_CO2_Germany.csv", "path to the raw CO2 CSV")
	processedDir := flag.String("processed", "data/processed", "folder for the cleaned dataset")
	outDir := flag.String("out", "outputs", "folder for model and analysis outputs")
	testYears := flag.Int("test-years", 10, "number of final years held out for testing")
	cvYears := flag.Int("cv-years", 20, "number of rolling one-step-ahead backtest years")
	breakYear := flag.Int("break-year", 1990, "structural break year (German reunification)")
	forecastTo := flag.Int("forecast-to", 2030, "last year to project")
	flag.Parse()

	if err := run(*input, *processedDir, *outDir, *testYears, *cvYears, *breakYear, *forecastTo); err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}
}

func run(input, processedDir, outDir string, testYears, cvYears, breakYear, forecastTo int) error {
	// 1. load + validate 
	lr, err := data.LoadCSV(input)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded %d rows for %s (%s)\n", len(lr.Records), lr.Meta.CountryName, lr.Meta.IndicatorCode)

	// 2. clean + features 
	opt := data.DefaultCleanOptions()
	opt.BreakYear = breakYear
	rows, rep, err := data.Clean(lr.Records, opt)
	if err != nil {
		return err
	}
	if cvYears <= 0 || cvYears > len(rows)-10 {
		return fmt.Errorf("cv-years must be between 1 and %d", len(rows)-10)
	}
	if forecastTo <= rep.LastYear {
		return fmt.Errorf("forecast-to (%d) must be after the last data year (%d)", forecastTo, rep.LastYear)
	}
	cleanPath := filepath.Join(processedDir, "co2_germany_clean.csv")
	if err := data.WriteCleanTable(cleanPath, rows); err != nil {
		return err
	}
	if err := writeQualityReport(filepath.Join(outDir, "data_quality_report.txt"), input, lr, rep); err != nil {
		return err
	}
	fmt.Printf("Cleaned %d years (%d-%d): %d imputed, %d duplicates, %d gap years, YoY outliers flagged: %v\n",
		rep.RowsOut, rep.FirstYear, rep.LastYear, len(rep.ImputedYears), len(rep.DuplicateYears), len(rep.GapYearsFilled), rep.OutlierYears)

	// 3. models
	specs := model.Specs(rep.FirstYear, breakYear)
	cfg := analysis.Config{TestYears: testYears, CVYears: cvYears, ForecastTo: forecastTo, BreakYear: breakYear}
	evals, future, err := analysis.EvaluateAll(rows, specs, cfg)
	if err != nil {
		return err
	}
	train, test, _ := analysis.Split(rows, testYears)

	if err := writeModelComparison(filepath.Join(outDir, "model_comparison.csv"), evals, train, test); err != nil {
		return err
	}
	if err := writeCoefficients(filepath.Join(outDir, "coefficients.csv"), evals, train, rows); err != nil {
		return err
	}
	if err := writePredictions(filepath.Join(outDir, "predictions.csv"), rows, test, evals); err != nil {
		return err
	}
	base := rows[opt.BaseYear-rep.FirstYear].CO2
	if err := writeForecast(filepath.Join(outDir, "forecast.csv"), future, evals, base); err != nil {
		return err
	}

	// 4. year / period analysis 
	if err := writeGroups(filepath.Join(outDir, "decade_summary.csv"), analysis.ByDecade(rows)); err != nil {
		return err
	}
	periods := analysis.ByPeriod(rows)
	if err := writeGroups(filepath.Join(outDir, "period_trends.csv"), periods); err != nil {
		return err
	}
	falls, rises := analysis.Extremes(rows, 5)
	if err := writeExtremes(filepath.Join(outDir, "yoy_extremes.csv"), falls, rises); err != nil {
		return err
	}

	printSummary(evals, periods, rows, train, test, base)
	fmt.Printf("\nOutputs written to %s/ and %s\n", outDir, cleanPath)
	return nil
}

// writers

func writeQualityReport(path, input string, lr *data.LoadResult, rep data.CleanReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "DATA QUALITY REPORT\n===================\n")
	fmt.Fprintf(&b, "Source file:     %s\n", input)
	fmt.Fprintf(&b, "Country:         %s (%s)\n", lr.Meta.CountryName, lr.Meta.CountryCode)
	fmt.Fprintf(&b, "Indicator:       %s\n", lr.Meta.IndicatorName)
	fmt.Fprintf(&b, "Indicator code:  %s\n", lr.Meta.IndicatorCode)
	fmt.Fprintf(&b, "Rows read:       %d\n", rep.RowsIn)
	fmt.Fprintf(&b, "Year range:      %d-%d (%d years after cleaning)\n\n", rep.FirstYear, rep.LastYear, rep.RowsOut)
	fmt.Fprintf(&b, "Checks performed\n----------------\n")
	fmt.Fprintf(&b, "- Required columns present:                 yes\n")
	fmt.Fprintf(&b, "- Single country in file:                   yes\n")
	fmt.Fprintf(&b, "- Year parsed as integer (1800-2100):        yes\n")
	fmt.Fprintf(&b, "- CO2 parsed as float, non-negative:         yes\n")
	fmt.Fprintf(&b, "- Duplicate years:                          %s\n", listOrNone(rep.DuplicateYears))
	fmt.Fprintf(&b, "- Missing years inserted:                   %s\n", listOrNone(rep.GapYearsFilled))
	fmt.Fprintf(&b, "- Values imputed (linear interpolation):    %s\n", listOrNone(rep.ImputedYears))
	fmt.Fprintf(&b, "- YoY %% change outliers (robust z > 3.5):  %s  [flagged, NOT removed]\n", listOrNone(rep.OutlierYears))
	fmt.Fprintf(&b, "\nRow-level issues\n----------------\n")
	if len(lr.Issues) == 0 {
		fmt.Fprintf(&b, "none\n")
	}
	for _, is := range lr.Issues {
		fmt.Fprintf(&b, "line %d: %s\n", is.Line, is.Reason)
	}
	fmt.Fprintf(&b, "\nColumns dropped from the modelling table (constant for every row):\n")
	fmt.Fprintf(&b, "Country Name, Country Code, Indicator Name, Indicator Code\n")
	fmt.Fprintf(&b, "\nNot possible with this dataset: spatial (lat/long) matching and district-level\n")
	fmt.Fprintf(&b, "comparison - the file is a single national total per year.\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func listOrNone(ys []int) string {
	if len(ys) == 0 {
		return "none"
	}
	s := make([]string, len(ys))
	for i, y := range ys {
		s[i] = strconv.Itoa(y)
	}
	return strings.Join(s, ", ")
}

func writeModelComparison(path string, evals []analysis.ModelEval, train, test []data.Row) error {
	header := []string{"model", "formula", "train_years", "test_years",
		"train_rmse", "train_mae", "train_r2", "train_adj_r2", "durbin_watson",
		"test_rmse", "test_mae", "test_mape_pct", "test_bias",
		"backtest_1step_rmse", "backtest_1step_mae", "backtest_1step_mape_pct"}
	var out [][]string
	for _, e := range evals {
		out = append(out, []string{
			e.Spec.Name, e.Spec.Formula,
			fmt.Sprintf("%d-%d", e.TrainFit.Years[0], e.TrainFit.Years[len(e.TrainFit.Years)-1]),
			fmt.Sprintf("%d-%d", test[0].Year, test[len(test)-1].Year),
			data.F(e.Train.RMSE, 2), data.F(e.Train.MAE, 2), data.F(e.Train.R2, 4), data.F(e.AdjR2, 4), data.F(e.DW, 3),
			data.F(e.Test.RMSE, 2), data.F(e.Test.MAE, 2), data.F(e.Test.MAPE, 2), data.F(e.Test.Bias, 2),
			data.F(e.CV.RMSE, 2), data.F(e.CV.MAE, 2), data.F(e.CV.MAPE, 2),
		})
	}
	return data.WriteCSV(path, header, out)
}

func writeCoefficients(path string, evals []analysis.ModelEval, train, all []data.Row) error {
	header := []string{"model", "fitted_on", "term", "coef", "std_err", "t_stat", "p_value"}
	var out [][]string
	add := func(e analysis.ModelEval, f *model.Fitted, label string) {
		for i, term := range e.Spec.Terms {
			row := []string{e.Spec.Name, label, term, data.F(f.Coef[i], 6), "", "", ""}
			if f.OLS != nil {
				row[4], row[5], row[6] = data.F(f.OLS.SE[i], 6), data.F(f.OLS.TStat[i], 3), fmtP(f.OLS.PValue[i])
			}
			out = append(out, row)
		}
	}
	for _, e := range evals {
		add(e, e.TrainFit, fmt.Sprintf("train %d-%d", e.TrainFit.Years[0], e.TrainFit.Years[len(e.TrainFit.Years)-1]))
		add(e, e.FullFit, fmt.Sprintf("full %d-%d", e.FullFit.Years[0], e.FullFit.Years[len(e.FullFit.Years)-1]))
	}
	return data.WriteCSV(path, header, out)
}

func fmtP(p float64) string {
	if math.IsNaN(p) {
		return ""
	}
	if p < 1e-4 {
		return "<0.0001"
	}
	return strconv.FormatFloat(p, 'f', 4, 64)
}

func writePredictions(path string, rows, test []data.Row, evals []analysis.ModelEval) error {
	header := []string{"year", "actual_co2_mt", "split"}
	for _, e := range evals {
		header = append(header, slug(e.Spec.Name))
	}
	testStart := test[0].Year
	var out [][]string
	for _, r := range rows {
		split := "train"
		if r.Year >= testStart {
			split = "test"
		}
		line := []string{strconv.Itoa(r.Year), data.F(r.CO2, 4), split}
		for _, e := range evals {
			line = append(line, data.F(predFor(e, r.Year, testStart), 4))
		}
		out = append(out, line)
	}
	return data.WriteCSV(path, header, out)
}

// predFor returns the in-sample fit for training years and the hold-out
// forecast for test years.
func predFor(e analysis.ModelEval, year, testStart int) float64 {
	if year >= testStart {
		return e.TestPred[year-testStart]
	}
	for i, y := range e.TrainFit.Years {
		if y == year {
			return e.TrainFit.Pred[i]
		}
	}
	return math.NaN()
}

func writeForecast(path string, future []data.Row, evals []analysis.ModelEval, base float64) error {
	header := []string{"year"}
	for _, e := range evals {
		header = append(header, slug(e.Spec.Name)+"_mt", slug(e.Spec.Name)+"_pct_vs_1990")
	}
	var out [][]string
	for i, r := range future {
		line := []string{strconv.Itoa(r.Year)}
		for _, e := range evals {
			v := e.Forecast[i]
			line = append(line, data.F(v, 2), data.F(100*(v-base)/base, 1))
		}
		out = append(out, line)
	}
	return data.WriteCSV(path, header, out)
}

func writeGroups(path string, gs []analysis.GroupSummary) error {
	header := []string{"group", "start_year", "end_year", "n_years", "mean_mt", "min_mt", "min_year", "max_mt", "max_year",
		"start_mt", "end_mt", "change_pct", "slope_mt_per_year", "slope_se", "slope_p", "avg_annual_pct", "trend_r2", "mean_pct_vs_1990"}
	var out [][]string
	for _, g := range gs {
		out = append(out, []string{g.Label, strconv.Itoa(g.StartYear), strconv.Itoa(g.EndYear), strconv.Itoa(g.N),
			data.F(g.Mean, 2), data.F(g.Min, 2), strconv.Itoa(g.MinYear), data.F(g.Max, 2), strconv.Itoa(g.MaxYear),
			data.F(g.StartValue, 2), data.F(g.EndValue, 2), data.F(g.ChangePct, 2), data.F(g.SlopeMt, 3), data.F(g.SlopeSE, 3),
			fmtP(g.SlopeP), data.F(g.AnnualPct, 3), data.F(g.R2, 3), data.F(g.MeanVsBase, 2)})
	}
	return data.WriteCSV(path, header, out)
}

func writeExtremes(path string, falls, rises []data.Row) error {
	header := []string{"type", "rank", "year", "co2_mt", "yoy_change_mt", "yoy_pct", "outlier_flag"}
	var out [][]string
	for i, r := range falls {
		out = append(out, []string{"largest_fall", strconv.Itoa(i + 1), strconv.Itoa(r.Year), data.F(r.CO2, 2), data.F(r.YoYChange, 2), data.F(r.YoYPct, 2), data.B(r.OutlierYoY)})
	}
	for i, r := range rises {
		out = append(out, []string{"largest_rise", strconv.Itoa(i + 1), strconv.Itoa(r.Year), data.F(r.CO2, 2), data.F(r.YoYChange, 2), data.F(r.YoYPct, 2), data.B(r.OutlierYoY)})
	}
	return data.WriteCSV(path, header, out)
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastUnderscore := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// console

func printSummary(evals []analysis.ModelEval, periods []analysis.GroupSummary, rows, train, test []data.Row, base float64) {
	fmt.Printf("\nMODEL COMPARISON (train %d-%d, hold-out %d-%d)\n", train[0].Year, train[len(train)-1].Year, test[0].Year, test[len(test)-1].Year)
	fmt.Printf("%-24s %9s %8s %7s %10s %10s %12s\n", "model", "trainRMSE", "trainR2", "DW", "testRMSE", "testMAPE%", "backtestRMSE")
	sorted := append([]analysis.ModelEval(nil), evals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Test.RMSE < sorted[j].Test.RMSE })
	for _, e := range sorted {
		fmt.Printf("%-24s %9.2f %8.4f %7.3f %10.2f %10.2f %12.2f\n", e.Spec.Name, e.Train.RMSE, e.Train.R2, e.DW, e.Test.RMSE, e.Test.MAPE, e.CV.RMSE)
	}
	fmt.Printf("\nPERIOD TRENDS\n")
	for _, p := range periods {
		fmt.Printf("%-40s slope %+7.2f Mt/yr  avg %+6.2f %%/yr  change %+6.1f %%\n", p.Label, p.SlopeMt, p.AnnualPct, p.ChangePct)
	}
	last := rows[len(rows)-1]
	fmt.Printf("\n%d emissions: %.1f Mt (%.1f%% vs 1990 level of %.1f Mt)\n", last.Year, last.CO2, last.PctVsBase, base)
}
