# NGDS Assignment 2 — Germany CO2 emissions regression in Go

Module 6FTC2177 Next Generation Data Science · Loh Zi Jian

A Go pipeline that loads, validates and cleans the World Bank series
**Carbon dioxide (CO2) emissions (total) excluding LULUCF (Mt CO2e)**
(`EN.GHG.CO2.MT.CE.AR5`) for Germany for 1970–2022. It then fits and compares
six regression models, analyses emissions by decade and by historical period,
and projects emissions to 2030.

Only the Go standard library is used: no external dependencies, and `go run` works offline.

## Run

```bash
go run ./cmd/co2pipeline                 # uses data/raw/NGDS_CO2_Germany.csv
go test ./...                            # unit tests (OLS, metrics, cleaning)
```

Flags (all optional):

| flag | default | meaning |
|---|---|---|
| `-input` | `data/raw/NGDS_CO2_Germany.csv` | raw CSV |
| `-processed` | `data/processed` | folder for the cleaned dataset |
| `-out` | `outputs` | folder for model/analysis outputs |
| `-test-years` | `10` | final years held out (2013–2022) |
| `-cv-years` | `20` | rolling one-step-ahead backtest years (2003–2022) |
| `-break-year` | `1990` | structural break for the piecewise model |
| `-forecast-to` | `2030` | last projected year |

## Project layout

```
cmd/co2pipeline/main.go     orchestration, CSV/report writers, console summary
internal/data/              load.go  – header/type validation, row-level issue log
                            clean.go – de-dup, gap filling, interpolation, features, outlier flags
                            write.go – CSV helpers
internal/model/             ols.go     – OLS via Householder QR, SEs, t-tests
                            metrics.go – RMSE, MAE, MAPE, R², adj. R², Durbin–Watson
                            specs.go   – the six model specifications, fitting, forecasting
internal/analysis/          evaluate.go – chronological split, rolling backtest, forecast
                            periods.go  – decade/period summaries, YoY extremes
data/raw/                   original file (unchanged)
data/processed/             co2_germany_clean.csv
outputs/                    model_comparison.csv, coefficients.csv, predictions.csv,
                            forecast.csv, decade_summary.csv, period_trends.csv,
                            yoy_extremes.csv, data_quality_report.txt
```

## Pipeline

1. **Validate**: required columns are looked up by name (column order and a UTF-8 BOM don't matter). The file must contain one country only. Year must be an integer between 1800 and 2100, and CO2 a non-negative number. Bad rows are logged with their line number instead of crashing the program.
2. **Clean**: removes duplicate years (keeping the first valid value), inserts missing years, fills missing values by linear interpolation (nearest value at the edges), and flags every change. Unusually large year-on-year changes (robust z-score > 3.5) are *flagged, not removed*, because big drops like 2009 and 2020 are real events. The Germany file turned out complete: no missing values, gaps or duplicates. The cleaning logic is exercised by the unit tests.
3. **Features**: `t` (years since 1970), `log_co2`, `co2_lag1`, YoY change (Mt and %), `pct_vs_1990`, decade, period label, `post_1990` dummy.
4. **Models**: all estimated with the same OLS routine.

| model | formula | why |
|---|---|---|
| Naive (random walk) | CO2ₜ = CO2ₜ₋₁ | baseline any model must beat |
| Linear trend | CO2 = b0 + b1·t | simplest long-run trend |
| Quadratic trend | + b2·t² | tests whether the decline is accelerating |
| Piecewise (break 1990) | + b2·post + b3·post·(t−20) | separate level and slope after reunification |
| Log-linear (1990+) | ln CO2 = b0 + b1·t | constant % decline per year |
| AR(1) + trend | CO2ₜ = b0 + b1·CO2ₜ₋₁ + b2·t | persistence plus trend |

5. **Evaluation**: chronological split (train 1970–2012, test 2013–2022, never shuffled). The test window is forecast *recursively*, so lag models feed on their own predictions. There is also a rolling-origin backtest over 2003–2022: for each year, refit on all earlier years and predict one year ahead.
6. **Year/period analysis**: decade and period summaries with a trend slope (Mt/yr), average % change per year, and the largest year-on-year rises and falls.
7. **Forecast**: each model is refitted on 1970–2022 and projected to 2030, reported in Mt and as % vs 1990.

Correctness check: coefficients and standard errors for all models were compared with `numpy.linalg.lstsq` and match to 6+ significant figures.

## Notes for interpretation

- Train RMSE for the log-linear model covers 1990–2012 only, so it is not directly comparable with the other models' train RMSE. Compare models on test RMSE and backtest RMSE.
- Durbin–Watson is reported for the trend models. Values well below 2 mean autocorrelated residuals, so their p-values are optimistic. For the AR(1) model DW is biased towards 2 (use Durbin's h if asked).
- Period boundaries (1990, 2000, 2020) are analyst choices tied to reunification, the Renewable Energy Sources Act era, and COVID/energy crisis. They are not learned from the data.

## Limitations

- One national series (53 points). Spatial (lat/long) matching and district comparison from the brief are **not possible** with this dataset. Time is the only explanatory variable, so the models describe the trend and do not explain its causes (energy mix, GDP, weather).
- The hold-out window contains shocks (2019 coal decline, 2020 COVID, 2022 energy crisis) that no trend model can foresee.
- Forecasts assume past trends continue; no policy scenarios.

## Use of Generative AI

- I ran, tested and validated all code on my own machine, independently reproduced the main regression result (slope −9.262691), and made the final model choice myself.
- **Claude (Anthropic)** was used to generate and debug the Go code.
- **Ollama (gemma4:12b)** was used to learn Go and to check explanations, with my own experiments to validate them.

Full details, prompts and validation steps are in `PROMPT_LOG.md`.