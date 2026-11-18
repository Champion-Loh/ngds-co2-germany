// Package data handles loading, validating and cleaning the World Bank
// CO2 emissions CSV for a single country.
package data

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// RequiredColumns are the columns the raw file must contain.
// Column order in the file does not matter; we look them up by name.
var RequiredColumns = []string{
	"Country Name", "Country Code", "Indicator Name", "Indicator Code", "Year", "CO2_Mt_CO2e",
}

// RawRecord is one row of the raw file after type conversion.
// Value is NaN when the cell was empty or not a valid number.
type RawRecord struct {
	Line    int // line number in the source file (for error messages)
	Country string
	Code    string
	Year    int
	Value   float64
}

// Metadata holds the columns that are constant for the whole file.
// They are dropped from the modelling table but kept for documentation.
type Metadata struct {
	CountryName   string
	CountryCode   string
	IndicatorName string
	IndicatorCode string
}

// LoadIssue records a row-level problem found while reading the file.
type LoadIssue struct {
	Line   int
	Reason string
}

// LoadResult is everything the loader produces.
type LoadResult struct {
	Meta    Metadata
	Records []RawRecord
	Issues  []LoadIssue // rows that were kept but flagged, or dropped
}

// LoadCSV opens path and parses it with ParseCSV.
func LoadCSV(path string) (*LoadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()
	return ParseCSV(f)
}

// ParseCSV reads the raw CSV, validates the header, converts types and
// records any row-level problems instead of failing on the first one.
// It only returns an error for problems that make the whole file unusable
// (unreadable file, missing columns, no valid rows, mixed countries).
func ParseCSV(r io.Reader) (*LoadResult, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // allow ragged rows; we check length ourselves

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	idx, err := columnIndex(header)
	if err != nil {
		return nil, err
	}

	res := &LoadResult{}
	line := 1
	for {
		row, err := cr.Read()
		line++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if isBlank(row) {
			continue // trailing empty lines are common; skip silently
		}
		if len(row) < len(header) {
			res.Issues = append(res.Issues, LoadIssue{line, fmt.Sprintf("dropped: expected %d fields, got %d", len(header), len(row))})
			continue
		}

		get := func(col string) string { return strings.TrimSpace(row[idx[col]]) }

		year, err := strconv.Atoi(get("Year"))
		if err != nil || year < 1800 || year > 2100 {
			res.Issues = append(res.Issues, LoadIssue{line, fmt.Sprintf("dropped: invalid year %q", get("Year"))})
			continue
		}

		val := math.NaN()
		rawVal := get("CO2_Mt_CO2e")
		if rawVal == "" || strings.EqualFold(rawVal, "NA") || rawVal == ".." {
			res.Issues = append(res.Issues, LoadIssue{line, fmt.Sprintf("year %d: missing CO2 value (will be imputed)", year)})
		} else if v, err := strconv.ParseFloat(rawVal, 64); err != nil {
			res.Issues = append(res.Issues, LoadIssue{line, fmt.Sprintf("year %d: non-numeric CO2 %q (will be imputed)", year, rawVal)})
		} else if v < 0 {
			res.Issues = append(res.Issues, LoadIssue{line, fmt.Sprintf("year %d: negative CO2 %.3f is impossible (will be imputed)", year, v)})
		} else {
			val = v
		}

		rec := RawRecord{Line: line, Country: get("Country Name"), Code: get("Country Code"), Year: year, Value: val}
		res.Records = append(res.Records, rec)

		if res.Meta.CountryName == "" {
			res.Meta = Metadata{get("Country Name"), get("Country Code"), get("Indicator Name"), get("Indicator Code")}
		}
	}

	if len(res.Records) == 0 {
		return nil, errors.New("no valid data rows found")
	}
	for _, r := range res.Records {
		if r.Code != res.Meta.CountryCode {
			return nil, fmt.Errorf("line %d: file mixes countries (%s and %s); expected one country", r.Line, res.Meta.CountryCode, r.Code)
		}
	}
	sort.Slice(res.Records, func(i, j int) bool { return res.Records[i].Year < res.Records[j].Year })
	return res, nil
}

func columnIndex(header []string) (map[string]int, error) {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		// strip a UTF-8 byte-order mark that Excel sometimes adds
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF"))] = i
	}
	var missing []string
	for _, c := range RequiredColumns {
		if _, ok := idx[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required column(s): %s", strings.Join(missing, ", "))
	}
	return idx, nil
}

func isBlank(row []string) bool {
	for _, f := range row {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}
