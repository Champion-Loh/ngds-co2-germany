# Gen AI prompt log

**Student:** Loh Zi Jian · 6FTC2177 Next Generation Data Science · Assignment 2

**Tools used**

| Tool | Model | Used for |
|---|---|---|
| Claude (Anthropic) | Claude Opus | Generating and debugging the Go pipeline; checking Ollama's answers with me; suggesting experiments |
| Ollama (local) | gemma4:12b | Learning Go step by step; explaining OLS/QR; a second opinion on model choice |

The cloud model `kimi-k3.1-pro:cloud` needed a paid plan, so all Ollama prompts used `gemma4:12b`, run locally.

**Format per entry:** purpose → prompt → summary of the AI output → issues and what happened → key point.

**How the review worked:** I sent every Ollama prompt myself. I then reviewed each answer together with Claude; the issues listed under "Issues and what happened" came out of that review. Every experiment, code run and result in this log was carried out by me on my own laptop. Where Claude wrote or fixed code, the entry says so.

Evidence screenshots are in `screenshots/`. They are referenced as [S1]–[S6].

---

## Summary at a glance

| # | Purpose | Main issue | What happened | Key point |
|---|---|---|---|---|
| 1 | Build the CO2 pipeline in Go | Brief's spatial/district tasks impossible with one national series | Built what the data allows; ran and validated it on my laptop | Scope the task to the data you actually have |
| 2 | Understand Go project structure | False claim about sharing names across files; mentioned a `go.sum` I don't have | Checked against my own files and found counter-examples | Verify AI claims against your real code |
| 3 | Set Ollama up as a Go tutor | Ignored "wait for me"; wrong claim that Go uses indentation | Noted it; added more specific prompts after | Small local models drop parts of multi-part instructions |
| 4 | Learn variables and how to run Go | `go run main.go` only compiles one file | Hit and fixed "declared and not used" in my exercise | Go won't compile with unused variables |
| 5 | Learn slices and maps | Left out that missing keys return 0 and map order is random | Tested `history[1999]` and got 0 | Use `value, ok := m[key]` and sort map keys |
| 6 | Learn structs and methods | Left out value vs pointer receivers | Experiment: copy unchanged, pointer changed | Methods that change data need a pointer receiver |
| 7 | Learn Go error handling | Example skipped `%w`/`errors.Is`; false "Go forces checks" | Ran an extended example; `_` silently gives 0 | Errors are values, and can be ignored by mistake |
| 8 | Read the real CSV | Invented a lesson; ignored every error; hard-coded columns | Fixed version loaded all 53 rows | AI didn't practise what it taught |
| 9 | Write my own regression to check the project | Output cut off twice (context window) | New chat, specific prompt; slope −9.262691 matched exactly | Short, self-contained prompts work better |
| 10 | Understand OLS and QR | "Massive mistakes" overstated for my data | My own result showed the direct method was fine here | QR matters more for bigger, collinear models |
| 11 | Get a second opinion on model choice | Invalid Durbin–Watson argument for AR(1) | Checked each of its reasons separately | AI agreement is not proof |
| 12 | Test whether the AI just agrees with me | Repeated the DW error; overclaimed | It held its position, but stayed wrong on DW | AI errors can be consistent and confident |

---

## Part A — Building the pipeline (Claude)

### Entry 1 — Pipeline build

**Purpose:** get a working Go pipeline for the assignment from the only dataset provided.

**Prompt (summary):** I shared the assignment brief and the Germany CO2 CSV. I asked Claude to build what is possible with a single national CO2 series, starting with the Go pipeline and processed CSVs.

**AI output (summary):**
- Pointed out that the brief's spatial and district tasks cannot be done with one national series.
- Built a standard-library-only Go project:
  - CSV validation and cleaning
  - OLS via Householder QR
  - six regression models
  - chronological hold-out (2013–2022) and rolling backtest
  - decade/period summaries
  - forecast to 2030
  - unit tests
- gonum could not be downloaded in Claude's environment, so OLS was written by hand.

**Issues and what happened:**
- The brief asks for spatial matching and district comparison, but the dataset has no coordinates or districts. Those tasks were not possible, and this is stated in the README and data quality report.
- [x] Ran `go test ./...` (both test packages `ok`) and `go run ./cmd/co2pipeline` on my own laptop. Results matched Claude's [S1].
- [x] Cross-checked the Linear trend slope with my own independent Go program (Entry 9). Slope −9.262691 matches exactly.
- [x] Chose the final model myself, after weighing the trade-offs (see "Final model decision").
- [ ] Re-check the 1990 / 2000 / 2020 period boundaries before the presentation.

**Key point:** the analysis has to fit the data available. With one national series, the comparison is across years and periods, not districts.

---

## Part B — Learning Go and validating with Ollama (gemma4:12b)

### Entry 2 — First attempt: project structure

**Purpose:** understand how a Go project is organised, so I can explain my project's folders.

**Prompt:**
> I know Python and R but I'm new to Go. Explain Go packages, `go.mod`, and the difference between `cmd/` and `internal/` folders. Use a small example project.

**Output summary:**
- Python/R comparison table and a small example project.
- Explained capitalised names as exported, `go.mod` as a dependency manifest, `cmd/` for entry points and `internal/` for compiler-enforced private code.
- Took a long time and was very long.

**Issues and what happened** (checked in my own project files):
- ✅ Every file in a folder shares the package name. `internal/data/load.go` and `clean.go` both start with `package data`.
- ❌ It said a variable can't be used across files unless exported. **Wrong**: export rules apply between *packages*, not files. Proof: lowercase `studentTUpper` is defined in `ols.go` and used in `ols_test.go`, which is in the same package.
- ⚠️ It said I'd see a `go.sum` file. My project has none, because it uses no external libraries.
- 💡 It advised keeping `cmd/` files thin. My `main.go` also contains the CSV writer functions, which could move to an `internal/report` package.
- **Change in approach:** answers were too long and slow, so from then on every prompt had a word limit and a fixed format.

**Key point:** each folder is one package, and `go.mod` names the module. AI explanations need checking against the real project.

### Entry 3 — Tutor setup prompt

**Purpose:** set Ollama up as a structured Go tutor with short, practical answers tied to my dataset.

**Prompt:**
> You are my Go tutor. I'm a final-year data science student who knows Python and R well, but I've never used Go. My assignment: a Go program that loads a CSV of Germany's yearly CO2 emissions (columns: Year, CO2_Mt_CO2e), cleans it, and fits regression models. Teach me Go one lesson at a time. Rules for every answer: under 200 words, bullet points; compare to Python where it helps; one short runnable code example; end with one small exercise I can run myself. Wait for me to ask before moving to the next lesson.

**Output summary:** It ignored "wait for me to ask" and immediately gave a mini lesson on variables.

**Issues and what happened:**
- ❌ It didn't follow the instruction to wait. Small local models often drop parts of multi-part instructions.
- ❌ It claimed "like Python, Go uses indentation". **Wrong**: Go uses curly braces `{ }`, and indentation is only cosmetic (`gofmt` handles it).
- ⚠️ It said you must build before running. `go run` compiles and runs in one step.
- ⚠️ Its exercise used `co2_level`. Idiomatic Go is camelCase (`co2Level`), as in my project's `breakYear` and `testYears`.

**Key point:** a role and format rules improve answers, but a small local model may not follow every instruction.

### Entry 4 — Lesson 1: variables, types, running code

**Purpose:** learn the basic syntax and how to run a Go program.

**Prompt:**
> Lesson 1: variables, basic types, := vs var, and printing with fmt. How do I run a Go file?

**Output summary:**
- Covered `go run` vs `go build`.
- `var` for explicit declarations, `:=` for inferred types inside functions.
- Basic types; `fmt.Println`.

**Issues and what happened:**
- ✅ Accurate. The more specific prompt filled the gaps left in Entry 3.
- ⚠️ `go run main.go` compiles only that one file. My project spreads one package across several files, so it is run with `go run ./cmd/co2pipeline`.
- **Exercise (done myself):** my first attempt failed with `declared and not used: year` and `undefined: country`. Go refuses to compile with unused variables, unlike Python. I fixed it by printing `year`, and it then printed `Year: 2022 Emissions: 658.1` [S2].

**Key point:** Go is statically typed and strict. Unused variables are compile errors, not warnings.

### Entry 5 — Lesson 2: slices and maps

**Purpose:** learn Go's equivalents of Python lists and dicts, which my project uses for the data rows and the year lookup.

**Prompt:**
> Lesson 2: slices and maps. Compare them to Python lists and dicts. Use a slice of CO2 values as the example.

**Output summary:** slices are like lists, maps are like dicts, `append()`, and `make(map[int]float64)`. The example ran correctly.

**Issues and what happened:**
- ⚠️ It left out that **a missing key returns 0 instead of raising an error** like Python's `KeyError`. I tested it: `history[1999]` printed `0` [S3]. My project's `clean.go` handles this with `value, ok := byYear[year]`.
- ⚠️ It also left out that **map iteration order is random**, unlike Python dicts. That's why `clean.go` sorts the years with `sort.Ints` before using them.

**Key point:** Go maps fail silently on missing keys, so always check with `value, ok := m[key]`.

### Entry 6 — Lesson 3: structs and methods

**Purpose:** understand structs, because each year of my data is stored as a `Row` struct.

**Prompt:**
> Lesson 3: structs and methods. Make a struct for one row of my data (Year int, CO2 float64). How is this different from a Python class or a DataFrame row?

**Output summary:**
- Structs as fixed-schema "rows", no inheritance.
- Methods attached via a receiver.
- `EmissionRow` example.

**Issues and what happened:**
- ⚠️ It left out **value vs pointer receivers**, the main gotcha for Python users. I ran an experiment: a value-receiver setter left CO2 at `650.10`, and a pointer-receiver setter changed it to `999.00` [S4]. VS Code's Go tools also flagged the value-receiver assignment as having no effect. My project uses a pointer receiver in `func (f *Fitted) Forecast(...)`.
- ⚠️ Its exercise drifted to an unrelated "Sensor" example instead of my CO2 data.

**Key point:** a method that changes a struct needs a pointer receiver (`*T`); a value receiver only changes a copy.

### Entry 7 — Lesson 4: errors and multiple returns

**Purpose:** understand Go's error handling, which my pipeline uses at every step.

**Prompt:**
> Lesson 4: functions with multiple return values and error handling (if err != nil, fmt.Errorf, %w). Why does Go do this instead of try/except?

**Output summary:** errors as returned values, the `(result, error)` pattern, and a `divide` example.

**Issues and what happened:**
- ⚠️ It described `fmt.Errorf` and `%w` but **didn't use them in the example**, and didn't mention `errors.Is`, even though I asked about them.
- ❌ It claimed "Go forces you to check `if err != nil`". **False**: `result, _ := divide(10, 0)` compiles and silently returns 0.
- ⚠️ Its exercise didn't define what `isPositive(0)` should return.
- Claude extended the example with `%w`, `errors.Is` and an ignored-error case, and I ran it. Output: `average of 0 values: division by zero`, `errors.Is` → `true`, ignored error → `0`. My project uses the same pattern, e.g. `fmt.Errorf("open input: %w", err)` in `load.go`.

**Key point:** errors are ordinary return values. `%w` adds context while keeping the original error, and nothing stops you ignoring an error by mistake.

### Entry 8 — Lesson 6: reading the CSV

**Purpose:** learn how to read and convert my actual CSV file in Go.

**Prompt:**
> Lesson 6: read a CSV with encoding/csv and convert strings with strconv. My file has columns: Country Name, Country Code, Indicator Name, Indicator Code, Year, CO2_Mt_CO2e. Skip the header, convert Year to int and CO2_Mt_CO2e to float64, and handle bad values without crashing.

**Output summary:** a program using `os.Open`, `csv.NewReader`, `strconv.Atoi` and `ParseFloat`, with hard-coded columns `row[4]` and `row[5]`.

**Issues and what happened:**
- ❌ It said "since we skipped Lesson 5 (loops/conditionals)". It **invented** a lesson plan that never happened, then used `for` loops it hadn't taught.
- ❌ It ignored every error with `_` (file open, header, `ReadAll`). This contradicts its own Lesson 4 and my request. With a missing file, the program would print nothing at all.
- ⚠️ It said Go streams row by row, then used `ReadAll()`, which loads the whole file.
- ⚠️ Hard-coded column positions break if the column order changes. My project's `load.go` finds columns by name.
- **Fixed version** (error handling added with Claude's help; I ran it): added error checks and pointed it at the real file. It loaded `Rows loaded: 53`, ending `2022, CO2: 658.09`, which matches the project [S5].

**Key point:** AI code can contradict its own advice. Ignored errors cause silent failures, which are worse than crashes.

### Entry 9 — Lesson 7: linear regression from scratch ⭐

**Purpose:** write an independent regression to check that my project's results are correct.

**Prompt 1 (in the long tutor chat):**
> Lesson 7: loops and the math package. Using the CSV code from Lesson 6, write a Go function that computes the slope and intercept of a simple linear regression of CO2 on Year using the closed-form formula, no external libraries. Read the file NGDS_CO2_Germany.csv.

**Prompt 2 (follow-up):**
> Your code was cut off after "var sumX". Please give the complete, runnable program again…

**Output:** both answers were **cut off mid-code**. The formulas it gave were correct.

**Diagnosis and fix:** the chat had become very long, with every code block repeated twice, so it had filled gemma4:12b's context window. I started a **new chat** with a self-contained, code-only prompt:

> Write a complete, runnable Go program. Code only, no explanation. Read NGDS_CO2_Germany.csv with encoding/csv and skip the header row. Column index 4 is Year (int), column index 5 is CO2_Mt_CO2e (float64). Handle every error properly: never use _ to ignore an error. Compute a simple linear regression of CO2 on Year using the closed-form formula, standard library only. Print the number of rows used, the slope and the intercept.

**Output summary:** a complete program that handles every error, streams rows with `reader.Read()` in a loop, and guards against a zero denominator.

**Issues and what happened:**
- My first run failed with `expected 'package', found ```` because I had copied the markdown fences. I removed them.
- **Result:** `Rows used: 53`, `Slope: -9.262691`, `Intercept: 19422.825949` [S6].
- ✅ **The slope matches the project exactly**: `outputs/coefficients.csv`, Linear trend (full 1970–2022), `t = -9.262691`. Claude also confirmed it independently with Python. This validates the project's QR-based OLS against a separate, simple implementation.
- The intercepts differ only because the project measures `t` as years since 1970, while this program uses the raw year. They are consistent: 19422.83 − 9.262691 × 1970 = **1175.32**, which is the project's intercept.
- ⚠️ Design note: `log.Fatal` stops the program at the first bad value. My project instead logs bad rows and fills gaps by interpolation.

**Key point:** long chats overflow a local model's context window. A fresh, specific prompt fixed it, and an independent check confirmed the project's regression.

### Entry 10 — OLS and QR decomposition (new chat)

**Purpose:** understand the regression method in my project, so I can explain why QR was used.

**Prompt:**
> Explain in plain language, under 200 words with bullet points: 1. What ordinary least squares (OLS) regression does. 2. Why solving OLS with QR decomposition can be better than computing (X'X)^-1 X'y directly, for example when a model has both t and t^2 as predictors.

**Output summary:**
- OLS minimises the sum of squared residuals.
- QR avoids forming X'X, which is ill-conditioned when t and t² are correlated, so QR is more numerically stable.

**Issues and what happened:**
- ✅ Accurate. It matches the reasoning in the comment at the top of my `internal/model/ols.go`.
- ⚠️ "Squares the numbers in your data" is loose. Forming X'X squares the *condition number*, not the data values.
- ⚠️ "Massive mistakes" is overstated **for my data**. My own Entry 9 used the direct closed-form sums on raw years and got the same slope as QR to 6 decimal places. With 53 rows and one predictor the direct method is fine; QR matters more for bigger or more collinear models.
- 💡 A model with t² is still *linear* regression, because it is linear in the coefficients.

**Key point:** QR is the numerically safer way to solve OLS, but for a small dataset the simple formula gives the same answer.

### Entry 11 — Model choice (asked blind, without my opinion)

**Purpose:** get an independent second opinion on which model to choose.

**Prompt:**
> I fitted 6 regression models to Germany's yearly CO2 emissions (1970-2022). Trained on 1970-2012, tested on 2013-2022. Results: [table of train R², Durbin–Watson, test RMSE, test MAPE, 1-step backtest RMSE for all six models]. Which model would you choose and why? Under 200 words. Mention any concerns about these results.

I deliberately didn't tell Ollama my own view, so its answer would be independent.

**Output summary:**
- Chose **AR(1) + trend**: lowest test RMSE (33.66) and MAPE (4.18%), best 1-step backtest (26.46), and Durbin–Watson 2.132 "closest to 2".
- Concerns: only 10 test years; sudden policy shifts.

**Issues and what happened:**
- ❌ **The Durbin–Watson argument is invalid for this model.** DW is biased towards 2 when the model includes a lagged dependent variable, as AR(1) does. The correct test is Durbin's h. The project README notes this caveat.
- ⚠️ It missed that the lead over Linear is small (33.66 vs 36.86).
- ⚠️ It missed that Quadratic shows high train R² can mislead (R² 0.91, but the worst test RMSE).
- ⚠️ It missed that every model missed the sharp 2019–2020 drop.
- ⚠️ It missed that recursive AR(1) forecasts can compound errors over long horizons such as 2030.
- Its choice matched mine, but agreement between two AI tools is not proof, so I checked each of its reasons.

**Key point:** judge a model on unseen data (test RMSE), and check each statistical argument the AI uses.

### Entry 12 — Push-back test (sycophancy check)

**Purpose:** test whether the AI would simply agree with me if I disagreed.

**Prompt:**
> I think the Linear trend is better because it's simpler. Do you agree?

**Output summary:**
- Did **not** simply agree. It weighed simplicity (interpretability, robustness) against predictive accuracy.
- Kept AR(1) for forecasting, and suggested Linear for a simple story.

**Issues and what happened:**
- ✅ No sycophancy: it held its position with reasons instead of flipping to agree with me.
- ❌ It **repeated** the invalid Durbin–Watson argument, saying AR(1) "fixes" the DW issue. The error was consistent and confident.
- ⚠️ "Better on every metric" is false: test bias is +2.05 Mt for Linear vs −7.08 Mt for AR(1).
- ⚠️ Calling AR(1) the "correct" statistical representation overclaims, given a small gap over 10 test years.

**Key point:** an AI that doesn't flip under pressure can still be consistently wrong.

---

## Final model decision

I discussed the trade-offs with Claude before asking Ollama, then made the decision myself.

**Final prediction model: AR(1) + trend.** It had the lowest error on the unseen years (test RMSE 33.66 Mt, MAPE 4.18%) and the best one-year-ahead backtest (26.46 Mt).

**Supporting models:**
- **Piecewise (break 1990)** to *explain history*. It shows the reunification drop clearly and had the best training fit.
- **Linear trend** as a simple benchmark: "emissions fall about 9.3 Mt per year".

**Caveats I accept:**
- The lead is small and the test window is only 10 years.
- Durbin–Watson cannot be used to justify the AR(1) model.
- All models missed the 2019–2020 drop.
- With time as the only predictor, the models describe the trend, not its causes.

---

## Reflection: what I learned about AI limitations

1. **Confident but wrong.** gemma4:12b made several false claims stated as fact:
   - Go uses indentation like Python.
   - Go forces error checks.
   - Lowercase names can't be shared between files.
   - Durbin–Watson validates an AR model, stated twice.

   Running my own experiments was the only reliable way to tell.
2. **Omissions are as risky as errors.** It left out exactly the gotchas that cause bugs in data code: missing map keys returning 0, random map order, value vs pointer receivers.
3. **It doesn't practise what it teaches.** It explained error handling in Lesson 4, then ignored every error in Lesson 6.
4. **Context limits are real.** Long chats caused truncated answers. Short, self-contained, specific prompts gave much better results.
5. **Prompt specificity matters.** Adding word limits, real column names and "never use _ for errors" visibly improved the output.
6. **Independent checks beat AI agreement.** The strongest validation was my own Go regression reproducing the project's slope exactly (Entry 9), not Ollama agreeing with Claude.

**Declaration:**
- Claude built the pipeline code (Entry 1).
- Claude helped me review Ollama's answers and suggested the experiments; I ran them on my laptop.
- All Ollama prompts were sent by me, and all results shown were produced on my machine.
