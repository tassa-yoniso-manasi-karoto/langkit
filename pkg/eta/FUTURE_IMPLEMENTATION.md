# FUTURE IMPLEMENTATION: ETA vNext

## Goal
Move the item-bar ETA (`ETACalculator`) from rule-based uncertainty bands to
calibrated prediction intervals, while preserving the existing architecture:

1. `Provider` interface remains unchanged.
2. Two calculators remain (`SimpleETACalculator` for media-bar,
 `ETACalculator` for item-bar).
3. Range display remains supported (`ETA: Xm-Ym`).
4. Conservative bias remains intentional.

This is a future roadmap, not a current implementation task.

## Why revisit later
Current ETA behavior is practical and stable, but its uncertainty range is
heuristic (constant/rule driven), not probabilistic. That is acceptable for
now, but it limits interpretability:

1. A displayed range is not guaranteed to be a true 95% interval.
2. Reliability percentages are not empirically calibrated coverage.
3. Formatter-level visual clamping can improve UX but decouples display from
 strict statistical meaning.

## Target state (long-term)
For advanced ETA only, display range as a prediction interval (PI) over
remaining completion time.

1. Point estimate: median predicted remaining time.
2. Range: quantile band (ex: 10%-90% or 2.5%-97.5%).
3. Optional label in dev mode: interval level (ex: `90% PI`, `95% PI`).
4. Reliability score derived from empirical calibration, not just interval width.

## Proposed model (advanced calculator)

### 1) Data representation
Maintain a rolling window of recent progress samples:

1. Timestamp.
2. Absolute completed tasks.
3. Optional per-update metadata (worker count, phase markers) if available.

Derive robust per-window rates using head-to-tail slope and trimmed pairwise
rates (avoid per-event micro-interval noise).

### 2) Forecast engine
Use lightweight bootstrap-style simulation on recent rates:

1. Build an empirical distribution of effective rates from the window.
2. Simulate many futures for remaining tasks (ex: 200-1000 draws).
3. Convert each simulated trajectory to remaining time.
4. Use quantiles for point/range output.

This gives a PI with direct semantics: observed completion should fall inside
the selected interval about that percentage of the time (after calibration).

### 3) Conservative policy
Conservatism should be explicit and bounded:

1. Shift point estimate to median or upper-median quantile (ex: p55/p60).
2. Optionally widen upper tail slightly in early phase.
3. Avoid unbounded multiplicative pessimism stacking.

### 4) Transition policy (warm-up)
Before sample quality is sufficient, keep heuristic fallback:

1. Require minimum elapsed time.
2. Require minimum effective sample count.
3. Require non-pathological variability window.
4. Until gates pass, emit heuristic ETA or no ETA.

Once gates pass, switch to PI engine.

## Formatter policy (future)
Formatter should stay presentation-only:

1. Consume calculator bounds as source of truth.
2. Decide point-vs-range from relative width.
3. If applying visual clamps, clearly treat them as display-only behavior.
4. Avoid re-deriving confidence with independent heuristics.

## Calibration plan (what "95%" must mean)

### 1) Logging requirements
For each emitted advanced ETA snapshot, log:

1. Timestamp.
2. Percent done.
3. Estimate/lower/upper.
4. Interval level used.
5. Variability and sample count.
6. Job/run identifier.

At run end, log actual completion time.

### 2) Offline evaluation
Compute empirical coverage by bucket:

1. Global coverage.
2. Early/mid/late progress buckets.
3. Low/high variability buckets.
4. Small/large task-count buckets.
5. Coarse throughput/hardware buckets (for example: low/medium/high observed
 early-run throughput), not per CPU/GPU model tables.

Coverage target examples:

1. `90% PI` should cover around 88-92%.
2. `95% PI` should cover around 93-97%.

### 3) Recalibration knobs
If miscalibrated, adjust:

1. Quantile mapping.
2. Effective sample weighting.
3. Early-phase fallback duration.
4. Tail expansion factors.

### 4) Stratification policy for hardware variability

1. Do not maintain per-device calibration sets (for example per CPU/GPU model).
2. Prefer online per-run adaptation as the primary mechanism.
3. If stratification is needed, use coarse runtime buckets derived from observed
 behavior (throughput/variability) rather than hardware brand/model metadata.
4. Keep the number of strata small and stable so calibration remains maintainable.

## Rollout strategy

### Phase A: Internal only (dev mode)

1. Keep existing ETA visible to users.
2. Compute PI ETA in parallel for shadow evaluation.
3. Log both outputs and compare.

### Phase B: Controlled enablement

1. Feature flag for PI ETA on item-bar.
2. Fallback to heuristic ETA on poor sample quality.
3. Continue calibration tracking.

### Phase C: Default

1. PI ETA becomes default for advanced calculator.
2. Heuristic path remains as safety fallback.

## Guardrails

1. Never emit negative durations.
2. Never emit inverted bounds.
3. Enforce minimum practical bound separation for readability.
4. Preserve stable behavior under `UpdateTotalTasks` and resumed runs.
5. Keep per-update compute cost bounded (no heavy simulation explosions).

## Non-goals for this future work

1. Do not merge simple and advanced calculators.
2. Do not change `Provider` interface contract.
3. Do not remove range display capability.
4. Do not require full statistical purity at the cost of unusable UX.

## Suggested acceptance criteria (future)

1. ETA remains stable under bursty concurrency.
2. ETA adapts to slowdowns within a few updates.
3. Displayed ranges stay actionable (not routinely 5x wide).
4. Empirical interval coverage is close to configured targets.
5. No regression in CLI/GUI performance.

<!--
What’s implemented now is a heuristic uncertainty band, not a statistical 85/95/98% interval:

1. In pkg/eta/advanced.go, bounds are built as estimate * (1 ± uncertainty) where uncertainty is hand-shaped from progress/sample-count/variability.
2. ReliabilityScore is then derived from interval width (1 - relativeWidth), not from probability coverage.
3. In internal/core/handler.go, very wide ranges are visually clamped to ±40% for display, which is useful UX-wise but breaks strict probabilistic meaning.

So your “85/95/98” concept is valid, but it would require a real probabilistic interval model.

Could it be implemented? Yes, within current architecture.
Best framing is a prediction interval (for remaining completion time), not a classical confidence interval on a mean.

Minimal viable approach:

1. Build rate samples from the sliding window.
2. Bootstrap/resample rates to generate many ETA outcomes.
3. Use quantiles for chosen level (e.g., 95% => 2.5th–97.5th percentiles).
4. Display as ETA: X-Y (95% PI).

If you want true statistical semantics, you should also avoid display clamping (or clearly label clamped output as “visual range”, not 95% interval).

■ Failed to paste image: no image on clipboard: The clipboard contents were not available in the requested format or the clipboard is empty.


› "Best framing is a prediction interval (for remaining completion time), not a classical confidence interval on a mean."
what does that mean? what is the difference?
"heuristic uncertainty band" no idea what that means. heuristic based on what?


• Difference in plain terms:

1. Confidence interval (CI): uncertainty about a parameter, like the true average processing rate.
2. Prediction interval (PI): uncertainty about a future outcome, like how long the remaining work in this run will take.

For ETA, you care about the second one.
You want “where will completion time land,” not “what is the true mean rate.”

Why PI is usually wider:

1. CI includes uncertainty in estimating the mean.
2. PI includes that plus run-to-run noise/jitter in actual task durations.

About “heuristic uncertainty band”:

It means the range is made from hand-tuned rules, not calibrated probability math.

In your current code:

1. It computes uncertainty from fixed rules based on progress, sample count, and variability (pkg/eta/advanced.go:173).
2. Then bounds are estimate * (1 ± uncertainty) (pkg/eta/advanced.go:198).
3. “Reliability” is derived from interval width, not statistical coverage (pkg/eta/advanced.go:207).
4. Formatter may clamp very wide ranges visually (internal/core/handler.go:968).

So yes, “heuristic” here means “rule of thumb from chosen constants,” not “this is a 95% interval.”


› why do ppl use heuristics instead? because prediction interval model are too conservative? Actually, I don't really remember exactly the details of how this advanced
algorithm was implemented originally, but I think there was kind of an attempt like this at first to change the way the ETA is calculated midway or something. I don't
fully remember, but basically, so we have this prediction interval model that we could implement and we have the heuristics. We could have the heuristics first and
then use the prediction interval model when the interval kind of narrows down, right? Again, I think this is kind of what was implemented before, wasn't it? (It was
probably implemented poorly, but...)


• People use heuristics mostly because they are easier to ship and control in product UX.

1. They are cheap and simple.
2. They do not need strong statistical assumptions.
3. They can be tuned to avoid ugly ranges early.
4. They let you encode product behavior directly (for example “always slightly pessimistic”).

Prediction intervals are not “too conservative” by nature.
They look wide when the process is genuinely noisy or when data is still sparse, which is often true early in your workload. That is usually honest uncertainty, not
necessarily a flaw.

Difference again, concretely:

1. Confidence interval: uncertainty about a parameter (like true average rate).
2. Prediction interval: uncertainty about a future observed outcome (your remaining completion time).

For ETA, PI is the right semantic target.

“Heuristic uncertainty band” in your current rewrite means the band is built from rules, not probabilistic coverage. It uses rules like:

1. Start with a base uncertainty.
2. Narrow with progress.
3. Widen with low sample count.
4. Widen with variability (CV).
5. Cap the max width.
6. Formatter may clamp very wide displayed ranges.

So yes, your hybrid idea is valid and common:

1. Warm-up phase: heuristic/no ETA until enough data.
2. Main phase: PI-based interval once sample quality is sufficient.
3. Optional smooth transition/blend to avoid a hard jump.

And yes, the old implementation did attempt mid-run regime changes, but it was still heuristic phase logic (sample/progress thresholds, cross-mult weights), not a true
PI model.


› "cheap and simple" "They do not need strong statistical assumptions." elaborate. what does this means in practice for langkit?


• In practice for Langkit, those phrases mean:

“Cheap and simple”

1. You can compute ETA from a few counters and a short window of samples.
2. Per update cost is low; no heavy resampling/simulation each progress tick.
3. Code is easier to debug and tune (a few constants, direct behavior).
4. Maintenance is lower because you don’t need statistical calibration infrastructure.

If you did a proper PI engine, you’d usually add more machinery: sampling/resampling, calibration checks, interval validation, fallback logic when sample quality is
poor.

“No strong statistical assumptions”
A true “95% interval” only means 95% if assumptions roughly hold. Typical assumptions are:

1. Recent samples are representative of near-future samples.
2. Noise pattern is reasonably stable.
3. Samples are not too dependent/pathological.
4. You have enough quality data.

Langkit often violates these:

1. Worker-pool burstiness creates correlated, irregular completions.
2. Item costs are multimodal (fast text-only vs slow FFmpeg-heavy lines).
3. Throughput shifts mid-run (thermal throttling, load, file-character changes).

So a mathematically labeled “95% PI” can be badly miscalibrated in real runs.
Heuristics avoid that hard probabilistic claim; they say “wider when noisy, narrower when stable.”

That’s the trade: heuristics give pragmatic behavior with fewer assumptions, but less statistical meaning.

■ Failed to paste image: no image on clipboard: The clipboard contents were not available in the requested format or the clipboard is empty.


› "Item costs are multimodal (fast text-only vs slow FFmpeg-heavy lines)." since ETA are created for a given progressbar which itself correspond to a given processing
type pre-determined for a processing run that doesn't sound like a problem. Or is it?
"Worker-pool burstiness creates correlated, irregular completions." doesn't it make it even worse for sliding windows based heuristics?
"sampling/resampling, calibration checks" "calibration infrastructure" elaborate, stop throwing keywords


• Good pushback. Concretely:

1. "Item costs are multimodal..."
   You’re right that ETA is per progress bar/process type. That reduces heterogeneity.
   It is still a problem in Langkit item-bar runs when, within that one run, items have different cost classes:

- short/easy lines vs long/complex lines
- lines that trigger heavier media work vs lines that don’t
- file-to-file shifts in bulk mode while item-bar keeps accumulating
  If your specific run is truly homogeneous, multimodality is much less of an issue.

2. "Worker-pool burstiness ... worse for sliding windows?"
   Yes for naive windows, no for robust windows.

- Bad: per-completion instantaneous rate (deltaTasks/deltaTime on tiny intervals) in a bursty system.
- Better: window head-to-tail slope (tasks_last - tasks_first) / (time_last - time_first), which averages bursts/gaps.
  So sliding window itself is not the problem; how rate is computed inside the window is.

3. "sampling/resampling, calibration checks, infrastructure" in plain Langkit terms
   If you want real 95% prediction intervals, practical version is:

- Sampling/resampling: keep recent item durations (or rates), simulate many possible futures, get quantiles.
- Example: run 500 simulations of remaining work, each gives a completion time.
- Use median as ETA point, and 2.5%/97.5% quantiles as 95% interval.
- Calibration checks: verify the claimed level matches reality.
- If you label ranges “95%”, then across many completed runs, actual finish should fall inside about 95% of those intervals.
- If it only happens 75-80%, your intervals are too narrow; if 99%, too wide.
- Calibration infrastructure: basic logging + analysis pipeline.
- Log each emitted ETA interval with context (progress, variability, remaining).
- Log actual completion time.
- Run a script to compute empirical coverage by bucket (early/mid/late progress, low/high variability).
- Adjust correction factors from that data.

Your hybrid idea is good, with one tweak: switch from heuristic to PI based on data quality gates, not on “interval already narrow” (that’s circular).
Use gates like minimum elapsed time, minimum effective samples, and stable variability.

-->
