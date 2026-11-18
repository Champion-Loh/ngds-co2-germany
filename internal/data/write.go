package data

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// WriteCSV writes a header and rows to path, creating parent folders.
func WriteCSV(path string, header []string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create folder for %s: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		f.Close()
		return err
	}
	if err := w.WriteAll(rows); err != nil { // WriteAll flushes
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}

// F formats a float for CSV output; NaN becomes an empty cell.
func F(v float64, decimals int) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// B formats a bool as 0/1.
func B(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// WriteCleanTable writes the processed dataset that is submitted as the
// "processed CSV" deliverable.
func WriteCleanTable(path string, rows []Row) error {
	header := []string{"year", "t", "co2_mt", "log_co2", "co2_lag1", "yoy_change_mt", "yoy_pct",
		"pct_vs_1990", "decade", "period", "post_1990", "imputed", "duplicate", "outlier_yoy"}
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = []string{
			strconv.Itoa(r.Year), F(r.T, 0), F(r.CO2, 4), F(r.LogCO2, 6), F(r.Lag1, 4),
			F(r.YoYChange, 4), F(r.YoYPct, 3), F(r.PctVsBase, 3), strconv.Itoa(r.Decade),
			r.Period, strconv.Itoa(r.PostBreak), B(r.Imputed), B(r.Duplicate), B(r.OutlierYoY),
		}
	}
	return WriteCSV(path, header, out)
}
