## 1. High-Level Overview

Langkit provides preflight validation tools so users discover media problems (corrupted files, missing tracks, broken subtitles, duration mismatches) before committing to a multi-file processing run.

*   **Expectation Checker (`internal/core/expectation*.go`)**: A preflight service that scans a directory/file and validates it against expected structures, user-defined language profiles, or auto-detected baseline consistencies.
*   **Integrity System (`internal/pkg/media/integrity_decode.go`)**: A deep-scan module that uses FFmpeg to decode media bitstreams and detect actual corruption (not just header checks).
*   **UI Model**: An "Issue-First" clustering approach with progressive disclosure. The Inspector Pane shows a quick summary in the **Preflight** tab and the user can open a full **Diagnostic Modal** for deep triage and profile management.


## 2. Modes of Operation

The Expectation Checker runs in two primary, combinable modes:

1.  **Auto Mode (Consistency Detection)**:
    *   Groups files by immediate parent directory.
    *   Builds a `DirectoryConsensus` based on the majority: languages are classified into quorum-tier (>= `QuorumPct`, default 75%) and soft-floor-tier (>= `SoftFloorPct`, default 20%). Audio track count uses the statistical mode; duration uses the median.
    *   Flags outliers: missing consensus languages, unusual track counts, duration outliers (Tukey fences, requires n >= 6).
    *   Ignores "bonus content" (trailers, specials, OVAs, etc.) using `bonusPattern` in `expectation_auto.go` -- all tokens (short and long) use word-boundary matching to avoid false exclusions on legitimate titles.
2.  **Profile Mode (Explicit Expectations)**:
    *   Validates files against a user-defined `ExpectationProfile` (saved in `~/.config/langkit/expectation_profiles.yaml` via `profiles.go`).
    *   Asserts specific audio/subtitle languages, requires video tracks, checks language tags, and checks external audio file durations.
    *   Has a dynamic "(From Settings)" profile that infers expectations from the user's current Langkit target/native language settings.

*Note: Structural checks (duration consistency, subtitle integrity, embedded/standalone overlaps) run unconditionally regardless of the chosen mode.*


## 3. Architecture & Backend Data Flow

### Entry Point

```go
RunCheck(ctx context.Context, rootPath string, profile *ExpectationProfile,
         autoConfig *AutoCheckConfig, decodeDepth media.IntegrityDepth,
         cb CheckCallbacks) (*ValidationReport, error)
```

Located in `internal/core/expectation_checker.go`. Takes six parameters -- not four. The `decodeDepth` parameter follows a resolution chain:

1. Explicit parameter (non-empty string)
2. `config.LoadSettings().IntegrityDecodeDepth` (user setting)
3. Fallback: `media.IntegritySampled`

### Execution Steps

1.  **Discovery**: Uses `DiscoverMediaFiles()` to find target media, mirroring the exact logic used by the main processing router.
2.  **Pass 1 (Probe)**: Iterates all files, extracts metadata via `Mediainfo()`, collects subtitle candidates via `CollectAllSubs(filePath, mi, true)` (retainUnknownLang=true), parses standalone subtitle files for line counts and tail coverage. Reports progress on `BarCheckProbe`. No heavy video decoding.
3.  **Structural Checks** (unconditional): Duration consistency between audio/video (tolerance from profile if available, else 2%), subtitle integrity (parse, empty, encoding, tail coverage), and embedded vs. standalone subtitle overlap.
4.  **Pass 2 (Profile/Decode)**:
    *   If a profile is given: decode integrity scoped to expected audio languages via `ResolveAudioStreamIndices()`, then domain checks (language presence, subtitle presence, language tags, external audio duration).
    *   If auto mode is enabled: a separate decode pass scoped to each directory group's preliminary consensus languages.
    *   If neither profile nor auto ran decode: a structural-only decode pass checks all audio streams.
    *   Deduplication inside `runDecodeIntegrity` ensures streams already checked by a prior pass are not re-decoded.
5.  **Auto Mode** (if enabled): Calls `runAutoMode()` which groups files by directory, builds the final `DirectoryConsensus` per group, and calls `runAutoChecks()` for consistency analysis. Reports progress on `BarCheckDecode`.
6.  **Post-Processing (Correlation)**: `mergeCorrelatedFindings()` scans the issue list. If an audio track has both a decode failure (`CodeAudioDecodeFailed`) and a duration deviation (`CodeAudioDurationMismatch`), both issues are **deleted** from the slice and **replaced** by a single `CodeCorruptTrack` issue that carries a human-readable message like "Audio (jpn) corrupt -- decoded 18m 3s of 24m 12s video". Severity counts are recalculated.


## 4. Issue Anatomy

Every finding is an `Issue` struct (`internal/core/expectation.go`):

```go
type Issue struct {
    Severity      Severity    // SeverityError, SeverityWarning, SeverityInfo
    Source        IssueSource // SourceProfile, SourceAuto, SourceStructural
    FilePath      string
    Category      string      // "integrity", "language", "duration",
                              // "subtitle", "structure", "consistency"
    Code          string      // stable grouping key (Code* constants)
    Message       string      // full human-readable message
    SubjectLabel  string      // bare language code, e.g. "tha", "ara-SA"
    MessagePrefix string      // text before the subject
    MessageSuffix string      // text after the subject
}
```

### Issue Codes (22 constants)

| Category | Codes |
|----------|-------|
| Structural | `mediainfo_failed`, `no_media_files`, `no_video_track`, `audio_decode_failed`, `video_decode_failed`, `corrupt_track`, `audio_duration_mismatch`, `ext_audio_duration`, `duration_unavailable` |
| Language | `missing_audio_lang`, `missing_sub_lang`, `untagged_track` |
| Subtitle | `sub_parse_failed`, `sub_empty`, `sub_encoding`, `sub_low_coverage`, `sub_overlap` |
| Auto | `auto_missing_audio`, `auto_missing_sub`, `auto_audio_count`, `auto_sub_count`, `auto_duration_outlier`, `auto_group_too_small` |

### The MessagePrefix/SubjectLabel/MessageSuffix Pattern

The backend pre-splits every issue message so the frontend renders `{prefix}<code>{subject}</code>{suffix}` with **zero string parsing**. The `SubjectLabel` carries only the bare language code (e.g. `"jpn"`, `"ara-SA"`), not a full label. Untagged tracks get an empty `SubjectLabel` (no code styling). This was introduced in commit 7965c70 to eliminate fragile `indexOf()`-based parsing in the frontend.


## 5. Language Matching & Subtitle Discovery

### Language Matching (BCP-47)

**Never use raw string comparison for languages.** Langkit supports scripts and regions (e.g., `zh-Hans`).

*   Always use `core.Lang` and the `langMatchesExpected(trackLang, expectedLang)` helper, which relies on `getIdx()` for script-aware matching.
*   For consensus counting, `langCode()` in `expectation_auto.go` collapses languages to ISO 639-3 while preserving vital subtags so that `zh-CN` and `zh-HK` do **not** collapse into a single `"zho"` bucket.

### Subtitle Discovery: retainUnknownLang

The checker calls `CollectAllSubs(filePath, mi, true)` at line 313 of `expectation_checker.go`.

*   Unlike the processing pipeline (which drops files it can't guess the language for), the checker **keeps them as `und` (undetermined)**.
*   This ensures the checker accurately counts every physical subtitle file on disk and can emit `CodeUntaggedTrack` warnings, rather than falsely claiming a file is missing entirely.

### Subtitle Coverage: Tail-Timestamp

Coverage is measured by **tail timestamp**, not cue-count ratio. `TailEndSec` is the median of the last `k = min(7, n)` cue end times, making it robust against a single outlier cue (e.g. a late credit). Tiered severity:

| Threshold | Severity |
|-----------|----------|
| tailGap > max(300s, 10% of video) | Info |
| tailGap > max(600s, 20% of video) | Warning |
| tailGap > max(900s, 30% of video) | Error |

Skipped entirely when video duration is unknown or cue count < 20.


## 6. The Media Integrity System

Located in `internal/pkg/media/integrity_decode.go`.

### Depth Levels

*   `sampled` (default): Seeks to 3 specific points (start, middle, near-end) and decodes a 20-second window per stream. Fast, catches most corruption. Falls back to a single start window if duration is unknown, or decodes the whole file if it is shorter than 60 seconds.
*   `full`: Decodes the entire audio stream end-to-end. Video is **always** sampled even in full mode.

### Mechanism

Uses `ffmpeg -hide_banner -v error -xerror -err_detect explode+crccheck -f null -`. Any output to stderr indicates bitstream corruption. Execution failures (FFmpeg binary missing) are surfaced as corrupted results with descriptive error output so they are never silently treated as clean.

### Scoping

`CheckDecodeIntegrity` only checks specific `StreamIndex` targets determined by the Expectation Checker via `DecodeScope { AudioStreamIndices, CheckVideo }`. For profile mode, streams are scoped to expected audio languages; for auto mode, to consensus languages; for structural-only, all audio streams. Already-checked streams are skipped via deduplication.

### Context Threading

The `ctx` parameter is threaded through to every FFmpeg command via `executils.CommandContext`, enabling cancellation of long-running decode operations when the user aborts a check.


## 7. Interpreted Summaries & Reporting

`GenerateInterpretedSummaries()` in `expectation_report.go` produces aggregated human-readable sentences from raw findings.

*   Groups issues by `(Code, Source)` key. Info-severity issues are excluded.
*   For language-related codes, attempts directory-level aggregation (e.g. "Directory S01: Missing expected audio language: Japanese") when all files in a directory share the same issue.
*   Returns `[]InterpretedSummary` -- a **struct** with `Source IssueSource` and `Message string`, not plain strings. This preserves the source tag for frontend grouping.
*   Sorted by source (profile -> structural -> auto), then alphabetically.
*   The `codeLabel()` function maps issue codes to human-readable cluster labels and **must stay in sync** with the frontend's `codeLabelMap` in `preflightDataUtils.ts`.


## 8. Frontend & UI Architecture

The UI (`internal/gui/frontend/src/components/`) emphasizes **"Triage over Reading"** using an Issue-First clustering model.

### State Management (`checkResultStore.ts`)

*   Manages the `CheckResultState`: `{ report, acknowledged, isRunning, stale, runToken }`.
*   **Run token**: A monotonic counter. `setRunning()` increments and returns a token; `setReport(token, report)` only writes if the token still matches. This prevents a stale in-flight result from overwriting a fresh one when the user changes settings mid-check.
*   **Stale detection**: `markStale()` sets `stale: true` on the current report without clearing it, so results remain visible with an amber banner prompting a re-run.
*   **CheckState** (derived store): `'unchecked' | 'running' | 'stale' | 'checked_clean' | 'checked_with_errors_unacknowledged' | 'checked_with_errors_acknowledged'`.

### Inspector Pane (App.svelte)

The Inspector Pane occupies the right-side panel and has a tabbed interface controlled by `inspectorMode: 'logs' | 'preflight'`:

*   **Tab bar**: A sliding toggle with an animated indicator that translates between the two tabs. The indicator is a `bg-primary/15 rounded-md` pill that smoothly transitions via CSS `transform: translateX(...)`.
*   **Preflight tab**: Renders `<PreflightDrawer>`.
*   **Logs tab**: Renders `<LogViewer>` in embedded mode (suppresses its own borders/shadow since the inspector wrapper provides them).
*   **Tab transitions**: Fade transitions (`duration: isLite ? 0 : 150`) between tab contents, respecting lite mode.

### UI Components

1.  **ActionBar (`ActionBar.svelte`)**:
    *   **Replaced** both the old PreflightBar and ProcessButton entirely (commit d34e39b). It is a single-row bar containing the check mode dropdown, the "Check Files" button, and the "Process Files" button.
    *   Subscribes to `invalidationErrorStore` to block processing on critical configuration errors (no media path, no features selected) and shows `ProcessErrorTooltip` on hover.
    *   The Process button changes to amber "Acknowledge & Process" when the check state is `checked_with_errors_unacknowledged`.
2.  **Preflight Drawer (`PreflightDrawer.svelte`)**:
    *   A progressive-disclosure pane inside the Inspector Pane's Preflight tab.
    *   Shows headline stat strip ("Telemetry": error/warning/info counts), consensus metadata as per-language chips, and severity-tinted cluster cards with glowing status dots.
    *   When results are stale, shows a thin inline amber banner (not a blocking overlay) so results stay visible and interactive.
3.  **Diagnostic Modal (`DiagnosticModal.svelte`)**:
    *   The power-user interface. Opened by clicking the telemetry strip or "View Details" in the drawer.
    *   **Left Pane**: Profile Manager for creating, editing, and deleting explicit expectation profiles. Includes a dropdown with "(From Settings)" + saved profiles + "+ New", inline editor fields, save/delete buttons (delete uses ConfirmDialog).
    *   **Right Pane**: Detailed Master-Detail Triage view. Groups findings by `IssueCode` (Clusters) in the master list; selecting a file shows its specific issues formatted as `{MessagePrefix}<code>{SubjectLabel}</code>{MessageSuffix}` directly from the struct fields.

### Data Clustering Utility (`preflightDataUtils.ts`)

*   **`getClusters(issues)`**: Groups the flat `ValidationIssue[]` by `issueCode` into `Cluster` objects. Each cluster has: `code`, `label` (from `codeLabelMap`), dominant `severity`, `source`, `category`, `fileCount`, and nested `ClusterFile[]`. Clusters are sorted by severity (error > warning > info), then by file count descending.
*   **`getTriageFiles(issues)`**: Groups by `filePath` into `TriageFile` objects. Each file has severity counts and a `severityScore` (errors*100 + warnings*10 + infos) for sorting. Files also get a `status` ('critical' / 'review' / 'clean') and a `topIssueSummary`.
*   **`codeLabelMap`**: A Record mapping all 22 issue codes to human-readable labels. Must stay in sync with `codeLabel()` in `expectation_report.go`.
*   **Null-safe**: Both `getClusters()` and `getTriageFiles()` accept `null | undefined` input and return `[]`.

### Profile Management

*   **Frontend**: `expectationProfilesStore.ts` -- a Svelte writable store backed by WebRPC calls to `ListProfiles`, `SaveProfile`, `DeleteProfile`. Uses `ensureLoaded()` for lazy initialization.
*   **Backend**: `profiles.go` -- standalone YAML persistence (not the global Viper/config singleton) at `~/.config/langkit/expectation_profiles.yaml`. All CRUD functions take a `zerolog.Logger` parameter. `SaveProfile` upserts by name; `DeleteProfile` errors if the profile doesn't exist.


## 9. The "Check-to-Process" Handshake

The system is designed so that checking is **always explicit** -- the user must click "Check Files" in the ActionBar. There is no silent auto-check on every process click (this was removed in commit d34e39b).

### Flow (managed by derived `checkState` in `checkResultStore.ts`)

1. **Unchecked/Clean**: The user can process freely. If a check has run and returned zero errors, processing proceeds normally.
2. **Checked with Errors (Unacknowledged)**: The Process button in ActionBar turns amber and its label changes to **"Acknowledge & Process"**.
3. **Acknowledgment**: Clicking the amber button acknowledges the errors, triggers `checkResultStore.acknowledge()`, and starts the processing run. No pop-up confirmation dialog is used for acknowledging errors (reducing friction). ConfirmDialog is only used for destructive actions like profile deletion.
4. **Stale State**: The frontend computes a fingerprint of the current settings (mode, profile, quorum, target/native languages). If any of these change after a check, `checkResultStore.markStale()` fires. The stale state shows a non-intrusive amber banner (not a blocking overlay) prompting a re-run, while keeping results visible and interactive.


## 10. Acknowledge & Skip (Preflight-to-Processing Bridge)

When the user acknowledges preflight errors and starts processing, the acknowledgement is scoped to **exactly what was reviewed** via a per-file skip list tied to a preflight fingerprint.

### Data Structures

```
struct SkipEntry
  - filePath: string
  - issueCodes: []string    # which codes were flagged for this file

struct AcknowledgedPreflight (on ProcessRequest)
  - preflightFingerprint: string   # sha256(rootPath + mode + profileHash
                                   #   + sorted(discoveredFilePaths))
  - acknowledgedSkips: []SkipEntry
```

### Backend Flow

1. When processing starts, files are re-discovered via `DiscoverMediaFiles()`.
2. The fingerprint is recomputed from the current discovery.
3. **Fingerprint matches**: For each file in `acknowledgedSkips`, if it appears in the discovery result, its listed issue codes are used to gracefully skip (via `Task.ShouldSkip(code)`) instead of `AbortAllTasks`. Files NOT in the skip list are processed normally.
4. **Fingerprint mismatch** (discovery drift): The skip list is discarded entirely. The user is warned that the library changed since the last check.

### Skip Code Mapping

| Preflight Code | Processing Failure Point | Skip Behavior |
|----------------|-------------------------|---------------|
| `missing_audio_lang` | `cards.go` ChooseAudio | skip file |
| `missing_sub_lang` | `routing.go` Autosub | skip file |
| `corrupt_track` | `routing.go` checkIntegrity | skip file |
| `audio_decode_failed` | `routing.go` checkIntegrity | skip file |
| `no_video_track` | various | skip file |

Codes like `sub_low_coverage`, `sub_encoding`, `auto_*` are informational and do not cause processing aborts, so they need no skip logic.


## 11. Gotchas & Pitfalls

### Never string-compare languages
Always use `langMatchesExpected()` with `Lang` structs. Raw string comparison will fail on script variants like `zh-Hans` vs `zho`.

### Nil slices encode as JSON null
Go encodes `nil []T` as `null`, not `[]`. The backend (`convertReport` in `internal/api/services/expectation.go`) pre-initializes all slice fields to `[]*generated.Type{}`. On the frontend, both `getClusters()` and `getTriageFiles()` guard against `null | undefined` input.

### Run token prevents stale overwrites
`checkResultStore` uses a monotonic counter (`runToken`) so that if the user starts a new check while an old one is still in-flight, the old result is silently discarded when it arrives. The `setReport(token, report)` call is a no-op if `token !== state.runToken`.

### Bonus content regex
`bonusPattern` in `expectation_auto.go` uses word-boundary matching for **all** tokens (short and long). Boundaries are: `string start/end` or separators (`. - _ [ ] ( ) space`). This prevents false exclusions from titles like "A Special Day" or "Disappearance".

### retainUnknownLang = true
The checker passes `true` to `CollectAllSubs()` so untagged subtitle files appear as `und` rather than being silently dropped. Without this, the checker would falsely report entire subtitle files as missing.

### Subtitle coverage uses tail-timestamp, not cue-count ratio
`TailEndSec` is the median of the last `k = min(7, n)` cue end times. This is robust against a single late outlier cue (e.g. a credit line extending past all dialogue). Thresholds were raised to avoid false positives on short episodic content.

### codeLabelMap must stay in sync
`codeLabel()` in `expectation_report.go` (Go) and `codeLabelMap` in `preflightDataUtils.ts` (TypeScript) both map the same 22 issue codes to human-readable labels. Adding a new code requires updating both.

### CheckCallbacks contract
`CheckCallbacks { Logger zerolog.Logger; OnProgress func(barID string, increment, total int, label string) }`. Progress bar IDs are `progress.BarCheckProbe` and `progress.BarCheckDecode`. Any new heavy loop in the checker **must** report progress through this callback.

### Pre-count decode work for accurate progress
`runDecodeIntegrity` pre-computes which files will actually be decoded (filtering out files with mediainfo errors, already-corrupted files, and already-checked streams) so the progress bar total reflects real work and always reaches 100%.
