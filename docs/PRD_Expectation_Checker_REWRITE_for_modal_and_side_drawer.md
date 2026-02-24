# Expectation checker: data quality + intent-based UI

## Context

Two problems prompted this work:

**Data quality:** Correlated findings (decode failure + duration deviation
on the same corrupt track) double the noise. `GenerateInterpretedSummaries`
groups by exact message text, so 20 corrupt tracks produce 20 separate
summary entries instead of "18 files have corrupt audio tracks." The GUI
shows two redundant layers (summaries vs per-file raw findings).

**UX architecture:** The checker is awkwardly shoehorned into a Feature
Card alongside processing features (subs2cards, dubtitles, etc.), causing
vertical clutter and a confusing mental model. When the user clicks
"Process", the system runs the check, stops, shows results, and forces
them to click "Process" again. Profile management is crammed inline.

**The pivot:** Adopt an intent-based UI that serves two user mindsets:
- **"Doer"** — just wants to process, only interrupted if something is
  broken. Served by a **Preflight Bar** + **Side Drawer**.
- **"Librarian"** — wants to audit and configure profiles. Served by a
  **Diagnostic Modal** opened via "Check Library".

---

# Part A: Backend — Issue Codes & Data Quality

## A1. Add issue codes to Issue struct

**File:** `internal/core/expectation.go`

Add `Code string` field to `Issue`. Define constants:

```go
const (
    CodeMediainfoFailed       = "mediainfo_failed"
    CodeNoMediaFiles          = "no_media_files"
    CodeNoVideoTrack          = "no_video_track"
    CodeAudioDecodeFailed     = "audio_decode_failed"
    CodeVideoDecodeFailed     = "video_decode_failed"
    CodeCorruptTrack          = "corrupt_track"
    CodeAudioDurationMismatch = "audio_duration_mismatch"
    CodeExtAudioDuration      = "ext_audio_duration"
    CodeDurationUnavailable   = "duration_unavailable"
    CodeMissingAudioLang      = "missing_audio_lang"
    CodeMissingSubLang        = "missing_sub_lang"
    CodeUntaggedTrack         = "untagged_track"
    CodeSubParseFailed        = "sub_parse_failed"
    CodeSubEmpty              = "sub_empty"
    CodeSubEncoding           = "sub_encoding"
    CodeSubLowCoverage        = "sub_low_coverage"
    CodeSubOverlap            = "sub_overlap"
    CodeAutoMissingAudio      = "auto_missing_audio"
    CodeAutoMissingSub        = "auto_missing_sub"
    CodeAutoAudioCount        = "auto_audio_count"
    CodeAutoSubCount          = "auto_sub_count"
    CodeAutoDurationOutlier   = "auto_duration_outlier"
    CodeAutoGroupTooSmall     = "auto_group_too_small"
)
```

Set code on every `AddIssue` call (~24 sites across
`expectation_checker.go` and `expectation_auto.go`).

## A2. Merge correlated decode + duration findings

**File:** `internal/core/expectation_checker.go`

Add `mergeCorrelatedFindings(report)` at the end of `RunCheck`, after
all checks complete. Logic:

- For each file with corrupted DecodeResults, map `StreamIndex` →
  audio track position via `mi.AudioTracks[i].StreamOrder`
- Find matching duration deviation issues (same file + track position)
- Remove both, emit single `CodeCorruptTrack` issue:
  `"Audio track 3 corrupt — decoded 1m 40s of 23m 54s video"`
- Unmerged decode/duration findings remain untouched

## A3. Improve GenerateInterpretedSummaries

**File:** `internal/core/expectation_report.go`

Change aggregation key from `(category, message, source)` to
`(code, source)`. Add `codeLabel()` helper for human-readable names.

For auto-mode language findings (where messages carry "present in N of
M files"), aggregate the stats: pick the message from the first file
since auto messages are identical for a given code+source, and prefix
with file count.

## A4. RIDL schema + codegen

**File:** `api/schemas/services/expectation.ridl`

Add `issueCode: string` to ValidationIssue struct. Run `make all` from
`api/` to regenerate Go + TypeScript.

## A5. API conversion layer

**File:** `internal/api/services/expectation.go`

Map `iss.Code` → generated `IssueCode` field in `convertReport`.

---

# Part B: Frontend — Intent-Based UI Restructure

## B1. Remove expectationCheck from FeatureSelector

**Files:** `featureModel.ts`, `App.svelte`

- Delete `expectationCheck` feature object from `featuresArray` in
  `featureModel.ts` (lines 522-544)
- Remove `expectationCheck: false` from `selectedFeatures` in App.svelte
- Remove `showProfileManager` reactive (line 118-119)
- Remove `expectationFingerprint` reactive (lines 534-539)
- Remove the `expectationCheck`-specific branching in `handleProcess()`
  (lines 780-806) — this logic moves to the new preflight flow
- Remove the `ConfirmDialog` for check results (lines 2130-2135)

Create standalone state variables in App.svelte to replace feature
options:

```typescript
let checkMode: string = 'auto';       // 'auto' | 'profile' | 'both'
let checkQuorum: number = 75;
let selectedProfileName: string = '';  // already exists
```

## B2. Create PreflightBar component

**New file:** `src/components/PreflightBar.svelte`

A compact horizontal bar at the bottom of the scrollable area, directly
above the fixed button row. Contains:

**Row 1 — Config:**
- Mode dropdown: `<select>` with options built from saved profiles:
  - `Auto (Consistency)` — auto mode, no profile
  - `(From Settings)` — builds a profile on-the-fly from the app's
    `targetLanguage` + `nativeLanguages` stores (reuses the existing
    `loadFromSettings()` logic from ProfileManager.svelte)
  - `Profile: <name>` — one entry per saved profile (profile-only mode)
  - `Both: <name>` — one entry per saved profile (auto + profile mode)
  - separator
  - `Manage Profiles...` — opens diagnostic modal
- If no saved profiles exist, show Auto + (From Settings) + Manage
- **Quorum slider:** When mode is Auto or Both, a compact inline
  slider (50-100%, step 5) appears next to the dropdown. Changing it
  marks `checkResultStore` stale.

**Row 2 — Actions:**
- `[🔍 Check Library]` secondary button — opens diagnostic modal,
  triggers a check run inside it
- `[▶ Process Files]` primary button — replaces current ProcessButton
  (or wraps it with new behavior)

**Props/events:** dispatches `process`, `checkLibrary`, emits
`modeChange(mode, profileName)`.

**Layout:** fixed bottom section in App.svelte, replaces current
ProcessButton row. The ProgressManager stays above it.

## B3. Inspector Pane (repurposed LogViewer panel)

**File:** `App.svelte` (layout), new `PreflightDrawer.svelte`

The existing right-side panel (`showLogViewer`, `w-[45%]`, slide-x)
becomes a unified **Inspector Pane** — NOT a new overlapping container,
but the same panel with a tab/state manager:

```typescript
let inspectorOpen: boolean = false;  // replaces showLogViewer
let inspectorMode: 'logs' | 'preflight' = 'logs';
```

- **Logs tab:** renders existing LogViewer content (unchanged)
- **Preflight tab:** renders a lightweight triage view:
  - Consensus pills at top (if auto mode)
  - Anomaly clusters (collapsible, using issue codes)
  - Compact — designed for quick scanning, not deep analysis

**Tab bar** at the top of the inspector pane: two tabs
(`Preflight | Logs`) allowing manual switching. System auto-switches:
- Check finds errors → open pane in `preflight` tab
- User acknowledges & processing starts → switch to `logs` tab
- User clicks log toggle button → open pane in `logs` tab

**Stale state overlay:** When `$checkResultStore.stale` is true, the
preflight tab content fades to 50% opacity with a centered overlay:
`"Results are out of date"` + `[↻ Re-run]` button. This prevents
debugging against stale data after profile edits or quorum changes.

**PreflightDrawer.svelte** — new component, receives
`$checkResultStore.report`, renders:
- Consensus pills (languages as `rounded-full` chips)
- Cluster view (shared `ClusterView.svelte` component)
- Stale overlay when applicable

## B4. ClusterView component (shared)

**New file:** `src/components/ClusterView.svelte`

Reusable component rendered in both the side drawer (compact) and the
diagnostic modal (full). Takes `issues: ValidationIssue[]` plus an
optional `compact: boolean` prop.

**Clustering logic:**
1. Group issues by `issueCode`
2. Sub-group by `filePath` within each code
3. Build cluster objects: `{ code, label, severity, source, fileCount,
   files: [{ path, name, issues }] }`
4. Sort: errors first → warnings → info; within severity, by file
   count descending

**Cluster labels** — static map from issue code to human name:
```
corrupt_track         → Corrupt Audio Tracks
audio_decode_failed   → Audio Decode Failures
audio_duration_mismatch → Audio Duration Mismatches
missing_audio_lang    → Missing Audio Languages
auto_missing_sub      → Missing Consensus Subtitles
auto_duration_outlier → Duration Outliers
...etc
```

**Rendering:**
- Each cluster: collapsible card with left-border severity stripe (2px,
  red/amber/blue), severity icon, label, file count badge
- Files listed in **sequential order** (alphabetical sort on path →
  preserves E01 → E02 → E03)
- Filenames in `font-mono text-xs`
- Per-file issue messages in `text-white/50`
- In compact mode (drawer): clusters closed by default, show only first
  5 files with "+N more" expand
- In full mode (modal): clusters open, all files shown

**Category filter chips** — toggle buttons for `category` values
(Integrity, Duration, Language, Subtitle, Consistency, Structure).
Only show chips for categories present in the report.

## B5. Diagnostic Modal

**New file:** `src/components/DiagnosticModal.svelte`

Full-screen overlay following the Settings modal pattern (backdrop blur,
centered panel, slide transition). Triggered by:
- "Check Library" button in PreflightBar
- "Manage Profiles..." dropdown item

**Two-pane layout:**

```
┌──────────────────────────────────────────────────────┐
│  Library Diagnostics                           [✕]   │
├──────────────┬───────────────────────────────────────┤
│              │                                       │
│  PROFILES    │  CHECK RESULTS                        │
│              │                                       │
│  [dropdown]  │  [consensus pills]                    │
│  [name    ]  │  [filter chips]                       │
│  [audio   ]  │  [cluster view - full mode]           │
│  [subs    ]  │                                       │
│  [✓ video ]  │  ...file-by-file details...           │
│  [✓ tags  ]  │                                       │
│  [dur tol ]  │                                       │
│  [quorum  ]  │                                       │
│              │                                       │
│  [Save]      │                                       │
│  [Delete]    │  [↻ Re-run Check]                     │
│              │                                       │
└──────────────┴───────────────────────────────────────┘
```

**Left pane:** ProfileManager (moved from main screen). Includes all
existing profile CRUD + the quorum slider and mode controls.

**Right pane:** Full ClusterView + consensus overview. Includes a
"Re-run Check" button that re-invokes `RunExpectationCheck` using the
current modal settings and refreshes the right pane.

**State:** The modal reads/writes the same `checkResultStore`. When
opened via "Check Library", it automatically runs a check.

**Stale handling:** Editing a profile or changing quorum in the left
pane calls `checkResultStore.markStale()`. The right pane immediately
fades to 50% opacity with a "Results are out of date" overlay and a
prominent `[↻ Re-run Check]` button — same treatment as the inspector
pane.

## B6. Rewire handleProcess flow

**File:** `App.svelte`

New `handleProcess()` logic:

```
1. User clicks Process in PreflightBar
2. If checkMode is set (auto/profile/both):
   a. Run check silently (show spinner on Process button)
   b. If ErrorCount === 0 → proceed to processing immediately,
      open inspector pane in logs tab
      (Warnings/Info do NOT block — per PRD, only Errors interrupt)
   c. If ErrorCount > 0 → open inspector pane in preflight tab,
      change button to "⚠️ Acknowledge Issues & Process" (amber)
   d. User clicks acknowledge → switch inspector to logs tab,
      start processing
3. If no check mode configured → start processing directly
```

**Critical rule: Errors-only blocking.** Warnings and Info findings
(e.g., duration anomalies, missing tags) must NOT interrupt the
processing flow. Only `report.errorCount > 0` triggers the halt +
acknowledge handshake. This matches the existing CLI behavior where
`--fail-on` defaults to `"error"`.

The `checkResultStore` state machine stays as-is. The acknowledge flow
reuses `checkResultStore.acknowledge()`.

The Process button state management:
- Normal: `▶ Process Files` (primary color)
- Check running: spinner + disabled
- Errors found: `⚠️ Acknowledge & Process` (amber)
- Processing: `Processing...` + spinner (existing behavior)

---

# Files to modify / create

| File | Change |
|------|--------|
| `internal/core/expectation.go` | Add Code field + constants |
| `internal/core/expectation_checker.go` | Set codes (~18 sites) + mergeCorrelatedFindings |
| `internal/core/expectation_auto.go` | Set codes (~6 sites) |
| `internal/core/expectation_report.go` | Group by Code in summaries |
| `api/schemas/services/expectation.ridl` | Add issueCode field |
| `internal/api/services/expectation.go` | Pass Code through convertReport |
| `internal/gui/frontend/src/lib/featureModel.ts` | Remove expectationCheck |
| `internal/gui/frontend/src/App.svelte` | Layout restructure, flow rewire |
| `internal/gui/frontend/src/components/PreflightBar.svelte` | **NEW** |
| `internal/gui/frontend/src/components/ClusterView.svelte` | **NEW** |
| `internal/gui/frontend/src/components/PreflightDrawer.svelte` | **NEW** |
| `internal/gui/frontend/src/components/DiagnosticModal.svelte` | **NEW** |
| `internal/gui/frontend/src/components/CheckResults.svelte` | Remove (replaced by ClusterView) |
| `internal/gui/frontend/src/components/ProfileManager.svelte` | Move into DiagnosticModal |

# Execution order

**Phase 1 — Backend (no frontend changes yet):**
1. Add Code field + constants to expectation.go
2. Set codes on all emission sites
3. Add mergeCorrelatedFindings
4. Update GenerateInterpretedSummaries
5. RIDL schema + codegen
6. API conversion
7. `go build ./...` — verify

**Phase 2 — Frontend scaffolding (Claude):**
8. Create empty shell components with correct props/events interfaces,
   placeholder markup, and all necessary imports:
   - `ClusterView.svelte` — props: `issues`, `compact`, events: none
   - `PreflightDrawer.svelte` — props: `report`, events: none
   - `PreflightBar.svelte` — props: `profiles`, `checkMode`, `quorum`,
     `selectedProfileName`, `isProcessing`, `checkState`, events:
     `process`, `checkLibrary`, `modeChange`
   - `DiagnosticModal.svelte` — props: `open`, `profiles`, `report`,
     events: `close`, `runCheck`, `saveProfile`, `deleteProfile`
9. Remove expectationCheck from featureModel.ts
10. Restructure App.svelte: remove old check/profile components from
    the scrollable area, wire in the new shell components at their
    correct layout positions, convert the LogViewer panel into the
    Inspector Pane with tabs (`inspectorMode`), add modal trigger
11. `cd internal/gui/frontend && npx vite build` — verify shells
    compile (components render placeholders, no logic yet)

**⏸ HANDOFF PAUSE — ChatGPT fills in component interiors:**
At this point Claude notifies the user that the shell components are
ready. The user then hands the plan + shell files to ChatGPT, who
drafts the full markup, styling, and reactive logic inside:
- ClusterView.svelte (clustering logic, rendering, filters)
- PreflightDrawer.svelte (consensus pills, stale overlay, compact
  cluster view)
- PreflightBar.svelte (mode dropdown with profiles + From Settings,
  inline quorum slider, action buttons)
- DiagnosticModal.svelte (two-pane layout, profile editor, dense
  results table, re-run button, stale overlay)

The user notifies Claude when ChatGPT's work is merged.

**Phase 3 — Frontend cleanup & integration (Claude):**
12. Review ChatGPT's output: fix TypeScript types, ensure store
    subscriptions follow existing patterns (no template literals,
    use string concatenation per CLAUDE.md), verify Svelte 4
    compatibility
13. Wire handleProcess flow: errors-only blocker rule, silent check,
    inspector pane auto-switching, acknowledge handshake
14. Wire stale state: quorum changes and profile edits call
    `checkResultStore.markStale()`
15. `cd internal/gui/frontend && npx vite build` — verify full build
16. End-to-end manual testing

# Verification

1. `go build ./...` — backend compiles
2. CLI: `./langkit-cli check <path> --auto` — merged findings, improved
   summaries
3. GUI: Preflight bar appears above process button area with mode dropdown
4. GUI: Click Process → check runs silently → if errors, drawer opens
   with cluster view → acknowledge → processing starts
5. GUI: Click Check Library → diagnostic modal opens → check runs →
   results appear in right pane → profile editing in left pane works
6. GUI: Drawer mode switching (preflight ↔ logs) works
7. GUI: Category + severity filters work in cluster view
