package data

import (
	"math"
	"strings"
	"testing"
)

const hdr = "Country Name,Country Code,Indicator Name,Indicator Code,Year,CO2_Mt_CO2e\n"

func row(year, val string) string {
	return "Germany,DEU,CO2 total,EN.X," + year + "," + val + "\n"
}

func TestParseRejectsMissingColumn(t *testing.T) {
	_, err := ParseCSV(strings.NewReader("Country Name,Year\nGermany,1990\n"))
	if err == nil || !strings.Contains(err.Error(), "missing required column") {
		t.Fatalf("expected missing-column error, got %v", err)
	}
}

func TestParseRejectsMixedCountries(t *testing.T) {
	in := hdr + row("1990", "100") + "France,FRA,CO2 total,EN.X,1991,50\n"
	if _, err := ParseCSV(strings.NewReader(in)); err == nil {
		t.Fatal("expected mixed-country error")
	}
}

func TestParseFlagsBadValuesAndYears(t *testing.T) {
	in := "\uFEFF" + hdr + row("1990", "100") + row("abc", "1") + row("1991", "") +
		row("1992", "n/a?") + row("1993", "-5") + row("1994", "90") + "\n"
	lr, err := ParseCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Records) != 5 { // bad year row dropped; bad values kept as NaN
		t.Fatalf("want 5 records, got %d", len(lr.Records))
	}
	if len(lr.Issues) != 4 {
		t.Fatalf("want 4 issues, got %d: %+v", len(lr.Issues), lr.Issues)
	}
	for _, y := range []int{1991, 1992, 1993} {
		if !math.IsNaN(lr.Records[y-1990].Value) {
			t.Errorf("year %d should be NaN", y)
		}
	}
}

func TestCleanInterpolatesGapsAndDuplicates(t *testing.T) {
	// 1990=100, 1991 missing value, 1992 missing row, 1993=70, duplicate 1993
	in := hdr + row("1990", "100") + row("1991", "") + row("1993", "70") + row("1993", "999") + row("1994", "60")
	lr, err := ParseCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	rows, rep, err := Clean(lr.Records, CleanOptions{BaseYear: 1990, BreakYear: 1990, OutlierThreshold: 3.5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("want 5 consecutive years, got %d", len(rows))
	}
	if rows[1].CO2 != 90 || rows[2].CO2 != 80 { // linear between 100 and 70
		t.Fatalf("interpolation wrong: 1991=%v 1992=%v", rows[1].CO2, rows[2].CO2)
	}
	if rows[3].CO2 != 70 || !rows[3].Duplicate {
		t.Fatalf("duplicate should keep first value 70 and be flagged, got %v", rows[3])
	}
	if len(rep.GapYearsFilled) != 1 || rep.GapYearsFilled[0] != 1992 {
		t.Fatalf("gap years: %v", rep.GapYearsFilled)
	}
	if !rows[1].Imputed || !rows[2].Imputed || rows[0].Imputed {
		t.Fatal("imputed flags wrong")
	}
	if !math.IsNaN(rows[0].Lag1) || rows[4].Lag1 != 70 || rows[4].YoYChange != -10 {
		t.Fatalf("lag features wrong: %+v", rows[4])
	}
	if rows[4].PctVsBase != -40 {
		t.Fatalf("pct vs base: got %v", rows[4].PctVsBase)
	}
}

func TestCleanEdgeImputation(t *testing.T) {
	in := hdr + row("1990", "") + row("1991", "80") + row("1992", "70") + row("1993", "")
	lr, _ := ParseCSV(strings.NewReader(in))
	rows, _, err := Clean(lr.Records, CleanOptions{BaseYear: 1991, BreakYear: 1990, OutlierThreshold: 3.5})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CO2 != 80 || rows[3].CO2 != 70 {
		t.Fatalf("edges should copy nearest value, got %v and %v", rows[0].CO2, rows[3].CO2)
	}
}

func TestCleanBaseYearOutOfRange(t *testing.T) {
	in := hdr + row("2000", "10") + row("2001", "11")
	lr, _ := ParseCSV(strings.NewReader(in))
	if _, _, err := Clean(lr.Records, DefaultCleanOptions()); err == nil {
		t.Fatal("expected error for base year 1990 outside 2000-2001")
	}
}
