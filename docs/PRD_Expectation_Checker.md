# PRD: Media Expectation Checker

## 1. Overview

The Expectation Checker is a **preflight validation service** that
verifies a media library's health before processing begins, catching
problems early (corrupted files, missing language tracks, broken
subtitles, mismatched durations) so users don't discover them halfway
through a multi-hour processing run.

It operates in two complementary modes:

- **Profile mode**: The user defines explicit expectations (expected
  audio/subtitle languages, structural rules) and the checker verifies
  reality against them.
- **Auto mode**: No user specification needed. The checker scans all
  files in a directory as a set, establishes what the norm is from the
  majority, and flags outliers. This is **internal consistency
  checking** -- if 23 out of 24 episodes have a Japanese audio track,
  the missing one is an anomaly.

Both modes can run independently or together.

## 2. Problem Statement

Today, Langkit performs only a basic FFmpeg header integrity check
(`checkIntegrity()` / `CheckValidData()`) on each video right before
processing it. This means:

- A corrupted file in a 50-file batch is only discovered when Langkit
  reaches it, potentially hours into a run.
- Missing or mis-tagged audio/subtitle tracks silently cause `Autosub()`
  failures or wrong-language selection, again discovered only at
  processing time.
- Subtitle files can be truncated or partially corrupted (e.g. a
  download interrupted at 30%), and this is never detected -- Langkit
  will produce a partial output without warning.
- External audio files that don't match the video duration indicate a
  mux/rip problem, but this is never checked.
- There is no way for the user to say "I expect every video to have a
  Japanese audio track and English subtitles" and have Langkit verify
  this upfront.

## 3. Goals

1. Allow users to define **expectations** about their media library
   (expected languages for audio/subtitle tracks, file structure
   assumptions).
2. **Automatically detect anomalies** within a directory by comparing
   files to their siblings -- no user specification required.
3. **Scan and validate** the entire target (file or directory tree)
   in a single pass *before* processing.
4. Produce a clear **report** with both raw findings and human-readable
   interpreted summaries, categorized by severity.
5. Expose this feature in both **CLI and GUI**.
6. Reuse and extend existing infrastructure (`Mediainfo()`,
   `CheckValidData()`, subtitle candidate collection, etc.).
7. Support **persistent, user-saved expectation profiles** for easy
   reuse across sessions and different media libraries.
8. Lay the architectural groundwork for future extensions (subtitle-
   audio alignment verification, library grooming).

## 4. Non-Goals (for initial release)

- Repairing or auto-fixing detected issues.
- Deep-scanning of video bitstreams frame-by-frame (full decode).
  The current FFmpeg header-probe approach is sufficient.
- Validating image-based subtitle formats (PGS, VobSub) beyond
  existence and language tagging.
- Network/remote media sources.
- Library grooming (see Future Development section).
- Subtitle-audio alignment verification via ASR (see Future
  Development section).

## 5. User-Facing Concepts

### 5.1 Expectation Profile

An **Expectation Profile** is a named, user-defined specification of
what a well-formed media item should look like. Profiles are persisted
to disk and selectable via a dropdown in the GUI.

| Field                       | Type           | Description |
|-----------------------------|----------------|-------------|
| `name`                      | `string`       | User-given profile name (e.g. "Thai anime", "JP drama"). |
| `expectedAudioLangs`        | `[]string`     | BCP 47 language tags the user expects to find as audio tracks (e.g. `["ja"]`). |
| `expectedSubtitleLangs`     | `[]string`     | Language tags expected for subtitles (embedded or standalone). |
| `requireVideoTrack`         | `bool`         | Whether each media file must contain a video track (default: true). |
| `requireLanguageTags`       | `bool`         | Whether audio/subtitle tracks must have language metadata (default: true). |
| `durationTolerancePercent`  | `float64`      | Allowed deviation in duration between audio tracks and video track, as a percentage (default: 2.0%). Combined with an absolute floor of 2 seconds to avoid noise on short media. |
| `subtitleLineThresholdPct`  | `float64`      | Minimum subtitle line count relative to the longest subtitle for the same media, expressed as a percentage. Files below this threshold are flagged as potentially corrupted (default: 80%). |
| `checkExternalAudioFiles`   | `bool`         | Whether to look for and validate external audio files alongside the video (default: false). |
| `videoExtensions`           | `[]string`     | Video file extensions to consider. Defaults to processing-parity mode (`.mp4`, `.mkv`). Broadening to `.avi`, `.mov`, etc. is an explicit opt-in. |

#### 5.1.1 Profile Persistence

Profiles are stored as YAML in the Langkit config directory
(`~/.config/langkit/expectation_profiles.yaml`), managed via direct
typed `yaml.Marshal` / `yaml.Unmarshal`. No Viper (which is bound
to the global `config.yaml` singleton). The file uses a `profiles:`
wrapper key for future extensibility (e.g. a `metadata:` section
for default profile selection):

```yaml
profiles:
  - name: "Thai anime"
    expectedAudioLangs: ["ja", "th"]
    expectedSubtitleLangs: ["ja", "th", "en"]
    requireLanguageTags: true
    subtitleLineThresholdPct: 75
  - name: "JP drama"
    expectedAudioLangs: ["ja"]
    expectedSubtitleLangs: ["ja", "en"]
    subtitleLineThresholdPct: 85
```

The Go struct for persistence wraps the slice:

```go
type ProfilesFile struct {
    Profiles []ExpectationProfile `yaml:"profiles"`
}
```

The GUI provides a dropdown to select a saved profile, plus buttons
to create, edit, and delete profiles. The CLI accepts a `--profile`
flag to load a saved profile by name.

### 5.2 Severity Levels

- **Error**: A condition that will certainly cause processing to fail
  or produce wrong output (e.g. corrupted video, expected audio
  language completely missing).
- **Warning**: A condition that *might* indicate a problem but could
  also be intentional (e.g. subtitle file has 15% fewer lines than a
  sibling, an audio track lacks a language tag).
- **Info**: Informational observations (e.g. "Found 3 audio tracks,
  2 subtitle files").

### 5.3 Auto Mode (Consistency Detection)

Auto mode requires **no user-defined profile**. Instead, it treats all
media files in a directory as a set and derives the "expected" state
from the majority. Deviations from this consensus are flagged as
anomalies.

**Core principle**: In a well-formed media library, sibling files
(episodes of the same season, files in the same directory) share a
common structure. They have the same audio languages, the same subtitle
languages, similar durations, and similar track counts. When a file
breaks this pattern, it's worth investigating.

#### 5.3.1 Consensus and Quorum

For each directory group, the checker collects a "fingerprint" of each
file (audio languages present, subtitle languages present, track
counts, duration). A property becomes part of the **consensus** when
it appears in at least `quorumPct`% of files in the group.

| Field        | Type      | Description |
|--------------|-----------|-------------|
| `quorumPct`    | `float64` | Minimum percentage of files that must share a property for a missing instance to be a **Warning** (default: 75%). |
| `softFloorPct` | `float64` | Minimum percentage for a missing instance to be **Info**. Properties below this threshold are suppressed entirely (default: 20%). |
| `minGroupSize` | `int`     | Minimum number of files to form a consistency group. Directories with fewer files are skipped for auto checks (default: 3). |

**Quorum rounding**: The required support count is
`ceil(quorumPct / 100 * n)`. This means with 75% quorum and 24
files, `ceil(0.75 * 24) = ceil(18.0) = 18` files are needed.
With 10 files: `ceil(0.75 * 10) = ceil(7.5) = 8`.

**Mode tie handling** (for track count checks in 6.5.4, 6.5.5):
If two or more values are tied for the mode, skip the check
for that property and emit no findings. A tie means there is no
clear norm to deviate from.

Example: A directory has 24 `.mkv` files. 23 have a Japanese audio
track. Quorum requires 18 files. Japanese audio is present in
23/24 (96%), so it reaches consensus. The 1 file missing it is
flagged.

#### 5.3.2 Directory Grouping

When scanning recursively, files are grouped by their **immediate
parent directory**. Each group gets its own independent consensus.
This is the natural boundary: `/Series/Season 1/` and
`/Series/Season 2/` are separate groups, because different seasons
may legitimately have different track configurations.

A single non-recursive scan (one directory) forms one group.

#### 5.3.3 Bonus Content Exclusion

Before computing consensus, files whose base name matches common
non-episode patterns are excluded from the consistency group.
These files still get integrity-checked (section 6.1) but do not
participate in or get flagged by auto consistency checks.

**Matching rules**: Patterns are case-insensitive. Long,
unambiguous tokens ("Extra", "Bonus", "Trailer", "Special",
"Preview") match as substrings. Short tokens that could collide
with legitimate episode names ("SP", "PV", "OVA", "OAD", "NCED",
"NCOP", "Menu") require **word-boundary matching** -- they must
be delimited by separators (`.`, `-`, `_`, `[`, `]`, `(`, `)`,
space, or string start/end). For example, `Series.SP01.mkv` is
excluded but `Disappearance.mkv` is not despite containing "sp".

#### 5.3.4 Relationship with Profile Mode

Auto mode and profile mode are **complementary**:

- **Profile only**: "Does my library match what I expect?"
- **Auto only**: "Is my library internally consistent?"
- **Both together**: Profile checks run first, then auto checks add
  additional findings. Issues carry a `source` field ("profile" or
  "auto") so the user can distinguish them.

Auto mode will never contradict a profile. If the user expects
Japanese audio via a profile and only 2 out of 24 files have it,
the profile says "22 files missing Japanese audio" (Error) while
auto mode has nothing to add (Japanese audio didn't reach quorum,
so it's not auto-expected).

## 6. Validation Checks

### 6.1 Video File Integrity

**Reuses**: `media.CheckValidData()` from `internal/pkg/media/ffmpeg.go`

- Run the existing FFmpeg header-probe on every video file.
- Report corrupted files as **Error**.
- This is already done per-file in `routing.go:checkIntegrity()` but
  the checker runs it upfront for the entire batch.

### 6.2 Track Existence and Language Tagging

**Reuses**: `core.Mediainfo()` from `internal/core/mediainfo.go`

For each video file, probe with mediainfo and check:

- **Video track present** (if `requireVideoTrack` is true).
- **Audio tracks**: For each entry in `expectedAudioLangs`, verify at
  least one `AudioTrack` matches. Report missing languages as
  **Error**.
- **Embedded subtitle tracks**: For each entry in
  `expectedSubtitleLangs`, verify a matching `TextTrack` exists OR a
  standalone subtitle file exists (see 6.4). Missing expected language
  with no fallback is an **Error**.
- **Untagged tracks**: Any audio or subtitle track with language `und`
  (undetermined) is a **Warning** if `requireLanguageTags` is true.

**Prerequisite refactoring**: `AudioTrack.Language` is currently
`*iso.Language` (no subtag/script support), parsed via bare
`iso.FromAnyCode()` at `mediainfo.go:205`. `TextTrack` already gets
the full `Lang` treatment via `ParseLanguageTags()` at line 219.
Audio tracks need the same upgrade to `Lang` with subtag support
before `getIdx()` script-aware matching can be used for audio
language expectations. Without this, BCP 47 tags like `zh-Hans`
would silently collapse and produce false "missing" reports.

### 6.3 Duration Consistency

**Reuses**: `AudioTrack.Duration` and `VideoTrack.Duration` from
mediainfo, plus `media.GetAudioDurationSeconds()` for external
files.

#### 6.3.1 Duration Normalization

Mediainfo reports durations as raw strings. The current
`GetAudioDurationSeconds()` scrapes ffmpeg stderr text output.
Both are fragile. The checker needs a robust normalization layer:

- `parseMediaInfoDuration(raw string) (float64, bool)` for
  mediainfo string values (handles formats like `"1234.567"`,
  `"N/A"`, empty strings).
- `probeDuration(path string) (float64, error)` using ffprobe
  machine-readable output (`-show_entries format=duration
  -of default=noprint_wrappers=1:nokey=1`), falling back to the
  existing `GetAudioDurationSeconds()` if ffprobe is unavailable.
- All comparisons in normalized `float64` seconds.
- If duration cannot be parsed for any track, emit **Info**
  ("duration unavailable for track N") rather than a spurious
  mismatch.

#### 6.3.2 Tolerance Model

A percentage-only threshold is noisy on short media (a 30-second
clip with a 1-second deviation is 3.3%). Use a hybrid threshold:

```
effectiveTolerance = max(
    videoDuration * durationTolerancePercent / 100,
    absoluteFloorSeconds,
)
```

- `durationTolerancePercent`: from profile (default 2.0%).
- `absoluteFloorSeconds`: hardcoded at 2.0s for v1.
- Deviation within `effectiveTolerance`: no issue.
- Deviation between `effectiveTolerance` and 10%: **Warning**.
- Deviation above 10%: **Error**.

#### 6.3.3 Checks

- Compare each audio track's duration to the video track's
  duration using the hybrid tolerance.
- For external audio files (if `checkExternalAudioFiles`), compare
  their duration to the video's.
- Optionally compare subtitle timespan coverage (last subtitle
  timestamp vs. video duration) -- a subtitle file whose last line
  ends more than 5 minutes before the video ends may be truncated.

### 6.4 Subtitle Validation

**Reuses**: Subtitle candidate collection logic (currently in
`cards.go:collectStandaloneCandidates()` and
`collectAllCandidates()`), embedded track metadata from mediainfo,
and `subs.OpenFile()` for parsing.

**Refactoring note**: `collectStandaloneCandidates()` and
`collectAllCandidates()` are methods on `*Task`. The checker needs
this logic but does not operate within a `Task`. These functions
should be extracted into standalone functions (or at minimum, the
core logic factored out) so both the checker and `Autosub()` can
call them. This is the same direction as extracting shared media
discovery (see section 7.2).

#### 6.4.1 Existence

- Collect both standalone subtitle files (using the existing
  filename-matching + language-guessing logic) and embedded
  `TextTrack` entries.
- For each `expectedSubtitleLang`, verify at least one candidate
  exists. Report missing as **Error**.

#### 6.4.2 Subtitle Integrity (text-based formats only)

For each text-based subtitle file (`.srt`, `.ass`, `.ssa`):

1. **Parse check**: Attempt `subs.OpenFile()`. If it fails, report
   as **Error** (unparseable / corrupted).
2. **Line count comparison**: Among all subtitle files for the
   *same* media file, compare line counts **within the same
   subtitle type group**. The existing `subtypeMatcher()` from
   `lang.go` classifies subtitles as CC, Dub, StrippedSDH, or Sub.
   CC/SDH subtitles naturally have more lines than dialogue-only
   subtitles, so comparisons should only be made within the same
   subtype. If a file has fewer than `subtitleLineThresholdPct`% of
   the maximum for its type group, flag as **Warning**.
3. **Empty file check**: A subtitle file with zero parseable lines
   is an **Error**.
4. **Encoding sanity**: If `OpenFile` succeeds but produces lines
   with a high proportion of replacement characters (U+FFFD) or
   null bytes, flag as **Warning** (likely encoding issue).

#### 6.4.3 Embedded vs. Standalone Comparison

If both an embedded subtitle track and a standalone file exist for
the same language:

- This is an **Info** observation (the user may have intentionally
  provided an external file to override the embedded one).
- If line counts differ significantly, note it as a **Warning**.

### 6.5 Consistency Checks (Auto Mode)

These checks are exclusive to auto mode. They operate on a set of
files and require no user-defined expectations -- the expectations
are inferred from the data.

#### 6.5.1 Audio Language Consensus

1. For each file in the group, collect the set of audio language
   codes (from `Mediainfo()` AudioTracks). Exclude `und` from
   consensus computation; untagged tracks are reported separately
   as metadata-quality issues.
2. For each language, compute its **confidence**:
   `confidence = supportCount / totalFiles`.
3. Classify missing instances by confidence tier:
   - `confidence ≥ quorumPct`: missing → **Warning**.
     "S01E14 is missing Japanese audio (present in 23/24 files)."
   - `softFloorPct ≤ confidence < quorumPct`: missing → **Info**.
     "S01E14 is missing Thai audio (present in 8/24 files)."
   - `confidence < softFloorPct`: suppressed (no finding).

This embraces newly discovered elements aggressively while
preventing singleton artifacts from generating noise. Auto mode
never uses Error severity because it is inferring expectations
rather than enforcing user-stated ones. Profile mode uses Error
for missing expected languages because the user has explicitly
stated what they want.

#### 6.5.2 Subtitle Language Consensus

Same approach as 6.5.1 for subtitles. Both embedded tracks and
standalone subtitle files count toward presence. Confidence
tiering applies identically (Warning above quorum, Info above
soft floor, suppressed below).

**Unknown-language handling**: Standalone subtitle candidates
whose language cannot be guessed from the filename are currently
dropped by `collectStandaloneCandidates()` (cards.go:67-71).
For the checker, these should instead be retained as `Lang=und`
and counted toward presence. An untagged subtitle file still
proves a subtitle exists for that video; the missing language
tag is reported as a separate metadata-quality **Info**.

#### 6.5.3 Duration Outlier Detection

Profile mode compares each audio track to its own video track
(section 6.3). Auto mode adds a **cross-file** comparison.

**Minimum sample size**: Duration outlier detection requires
**n ≥ 6** files in the group. Quartile estimates are unstable
with fewer data points. Groups with 3-5 files still get
language/track consensus checks but skip duration outliers.
For groups below the threshold, optionally emit **Info**:
"Duration outlier detection skipped for [dir] (N files,
minimum is 6)."

**For n ≥ 6:**

1. Collect all video durations in the group. Parse via
   `parseMediaInfoDuration()` (section 6.3.1).
2. Compute the **median**, **Q1**, **Q3**, and **IQR**
   (interquartile range: Q3 − Q1).
3. A file is an outlier if its duration falls outside the
   **Tukey fences**: below Q1 − 1.5×IQR or above Q3 + 1.5×IQR.
4. Outliers are **Warning**: "S01E03 duration (12m 34s) is
   unusually short compared to siblings (median: 23m 45s)."

**IQR = 0 fallback** (all files near-identical duration):

Use absolute + percentage floor:
`abs(di - median) > max(absFloor, pctFloor * median)`

- `absFloor`: 120 seconds (2 minutes).
- `pctFloor`: 5%.

This prevents flagging trivially small deviations while still
catching genuinely truncated or extended files.

#### 6.5.4 Audio Track Count Consistency

1. For each file, count the number of audio tracks.
2. Compute the mode (most common count).
3. If the mode appears in ≥ `quorumPct`% of files and a file has
   a different count, emit **Info**: "S01E07 has 3 audio tracks
   (most files have 2)."

This is informational, not a warning, because extra tracks are
rarely a problem. But a file with fewer tracks than the norm may
indicate a re-encode that dropped a track.

#### 6.5.5 Subtitle Count per Language Consistency

Within a consensus subtitle language, check whether the number of
subtitle sources (standalone + embedded) is consistent:

1. For each consensus language, count how many subtitle sources
   each file has for that language.
2. Compute the mode count.
3. Files deviating from the mode get **Info**: "S01E11 has 1
   English subtitle source (most files have 2)."

This catches cases where some files are missing an alternative
subtitle variant (e.g. a CC track that most other files have).

## 7. Architecture

### 7.1 Design Philosophy: Preflight Service, Not a Mode

The checker is **not** a new `Task.Mode`. The existing `Mode` enum
(`Subs2Cards`, `Subs2Dubs`, `Enhance`, `Translit`, `Condense`)
drives transformation pipelines: `Routing()` dispatches based on
mode, `Execute()` branches on mode, subtitle discovery and worker
pools are mode-gated. Shoehorning a diagnostic pass into this
pipeline would require touching many mode-conditional branches for
a feature that doesn't transform anything.

Instead, the checker is a **dedicated preflight service** with its
own entry point, its own WebRPC endpoint, and its own execution
flow. It is invoked *before* processing (optionally, by the user)
and returns a structured report. It does not go through `Routing()`
or `Execute()`.

This mirrors how `checkIntegrity()` already works -- it's a
function called within the routing flow, not a mode. The checker is
the same idea, expanded to a full pre-check and exposed as a
user-facing feature rather than an internal gate.

**Future baseline integration**: Eventually, the checker could be
called automatically at the start of `Routing()` (after file
discovery, before the per-file processing loop), using the user's
default profile. This would replace the current per-file
`checkIntegrity()` call with a comprehensive batch pre-check. The
preflight-service design makes this possible without architectural
changes -- it's just a matter of calling `RunExpectationCheck()`
from `Routing()`.

### 7.2 Shared Media Discovery

Currently, file discovery is inconsistent across the codebase:

- `routing.go:187` walks directories matching only `.mp4`/`.mkv`,
  skips `.media` dirs and merged outputs.
- `handler.go:1138` (`GetVideosInDirectory`) uses a broader set of
  extensions (`.mp4`, `.mkv`, `.avi`, `.mov`, `.wmv`, `.flv`,
  `.webm`, `.m4v`) with no skip rules.

The checker must discover **the same files that processing will
operate on**, otherwise the user checks one set of files and
processing runs another. To solve this:

- Extract the walk + filter logic from `routing.go:178-221` into a
  shared `DiscoverMediaFiles(path, extensions)` in
  `internal/core/` that both `Routing()` and the checker call.
  The function includes the same skip rules (`.media` dirs, merged
  outputs).
- **Strict parity by default**: the checker defaults to exactly
  the processing discovery set (`.mp4`, `.mkv`) with the same skip
  rules. Profile `videoExtensions` can broaden this as an explicit
  advanced option, but the user should understand that processing
  may not handle those extra extensions yet.
- This is a prerequisite refactoring, not optional.

### 7.3 Backend

```
internal/
  core/
    discovery.go            # Shared DiscoverMediaFiles(), extracted
                            # from routing.go walk logic
    expectation.go          # ExpectationProfile, ValidationReport,
                            # Issue types
    expectation_checker.go  # RunExpectationCheck() - core logic
                            # (profile checks)
    expectation_auto.go     # RunAutoCheck() - consensus analysis
                            # and consistency checks
    expectation_report.go   # Report formatting (CLI table, JSON,
                            # interpreted summaries)
  config/
    profiles.go             # Profile CRUD (yaml marshal/unmarshal)
  pkg/
    media/
      ffmpeg.go             # (existing) CheckValidData
      duration.go           # Duration normalization layer
  api/
    schemas/services/
      expectation.ridl      # WebRPC schema
  internal/api/
    services/
      expectation.go        # WebRPC service implementation
```

**Key types:**

```go
// ExpectationProfile defines what the user expects.
type ExpectationProfile struct {
    Name                    string   `yaml:"name"`
    ExpectedAudioLangs      []string `yaml:"expectedAudioLangs"`
    ExpectedSubtitleLangs   []string `yaml:"expectedSubtitleLangs"`
    RequireVideoTrack       bool     `yaml:"requireVideoTrack"`
    RequireLanguageTags     bool     `yaml:"requireLanguageTags"`
    DurationTolerancePct    float64  `yaml:"durationTolerancePercent"`
    SubtitleLineThresholdPct float64 `yaml:"subtitleLineThresholdPct"`
    CheckExternalAudio      bool     `yaml:"checkExternalAudioFiles"`
    VideoExtensions         []string `yaml:"videoExtensions"`
}

// Severity levels for findings.
type Severity int
const (
    SeverityError Severity = iota
    SeverityWarning
    SeverityInfo
)

// IssueSource distinguishes profile-based from auto-detected findings.
type IssueSource string
const (
    SourceProfile IssueSource = "profile"
    SourceAuto    IssueSource = "auto"
)

// Issue represents a single validation finding.
type Issue struct {
    Severity Severity
    Source   IssueSource
    FilePath string
    Category string // "integrity", "language", "duration",
                    // "subtitle", "structure", "consistency"
    Message  string // Human-readable interpreted message
    Details  map[string]interface{}
}

// AutoCheckConfig controls auto mode behavior.
type AutoCheckConfig struct {
    Enabled      bool    `yaml:"enabled"`
    QuorumPct    float64 `yaml:"quorumPct"`      // default: 75.0
    SoftFloorPct float64 `yaml:"softFloorPct"`   // default: 20.0
    MinGroupSize int     `yaml:"minGroupSize"`   // default: 3
}

// DirectoryConsensus captures the inferred "norm" for one directory.
type DirectoryConsensus struct {
    Directory         string
    FileCount         int
    BonusExcluded     int            // files excluded by 5.3.3
    AudioLangs        map[string]int // lang code → file count
    SubtitleLangs     map[string]int // lang code → file count
    AudioTrackCounts  map[int]int    // track count → file count
    SubCountPerLang   map[string]map[int]int // lang → count → files
    Durations         []float64      // video durations in seconds
    // Tiered language sets (derived from quorum/softFloor):
    QuorumAudioLangs  []string       // confidence ≥ quorumPct
    SoftAudioLangs    []string       // softFloor ≤ conf < quorum
    QuorumSubLangs    []string       // confidence ≥ quorumPct
    SoftSubLangs      []string       // softFloor ≤ conf < quorum
    ConsensusAudioCount int          // mode audio track count
    MedianDuration    float64
}

// ValidationReport is the complete output of a check run.
type ValidationReport struct {
    Profile              *ExpectationProfile // nil if auto-only
    AutoConfig           *AutoCheckConfig    // nil if profile-only
    RootPath             string
    TotalFiles           int
    Issues               []Issue
    InterpretedSummaries []string
    ErrorCount           int
    WarningCount         int
    InfoCount            int
    Duration             time.Duration
    FileMetadata         map[string]*FileCheckResult
    Consensus            map[string]*DirectoryConsensus // dir → consensus
}

// FileCheckResult stores all metadata gathered for a single
// media file during the check pass.
type FileCheckResult struct {
    VideoFile       string
    MediaInfo       MediaInfo
    AudioTracks     []TrackCheckResult
    SubtitleSources []SubSourceCheckResult
    Integrity       bool
}

type TrackCheckResult struct {
    Track       AudioTrack
    MatchedLang string // which expected lang it matched, "" if none
    IsExpected  bool
}

type SubSourceCheckResult struct {
    Source        SubtitleCandidate
    LineCount     int
    MatchedLang   string
    IsExpected    bool
    ParseErrorMsg string // serializable; "" if no error
}
```

**Execution flow:**

1. User provides a path (file or directory), optionally an
   `ExpectationProfile`, and optionally an `AutoCheckConfig`.
   Both may be nil -- in that case, only structural checks
   (integrity, duration consistency within each file) run.
2. `RunCheck(ctx, path, profile, autoConfig)` calls
   `DiscoverMediaFiles()` to collect all video files (same logic
   and skip rules as `Routing()`).
3. **Probe pass** (always runs): For each video file, run
   `Mediainfo()`, collect subtitle candidates, run integrity
   check. Populate `FileCheckResult` for every file. This pass
   produces structural findings (integrity errors, duration
   anomalies within a file) regardless of mode.
4. **Profile checks** (if profile is non-nil): For each file, run
   checks 6.1-6.4, comparing against the profile's expectations.
   Issues are tagged `source: "profile"`.
5. **Auto checks** (if autoConfig is non-nil):
   a. Group files by immediate parent directory.
   b. For each group with ≥ `minGroupSize` files, compute
      `DirectoryConsensus` from the probe data. Groups below
      the threshold emit an **Info**: "Skipped auto-check for
      [directory] (N files, minimum is M)."
   c. Run consistency checks 6.5.1-6.5.5 against the consensus.
      Issues are tagged `source: "auto"`.
6. Generate interpreted summary messages from all findings
   (see section 7.6).
7. Aggregate into a `ValidationReport`.
8. Return to the caller (CLI prints; GUI receives via WebRPC).

Note: the probe pass (step 3) runs exactly once regardless of
which modes are active. This means running both profile + auto
costs almost nothing extra compared to running one alone -- the
expensive work (mediainfo, ffmpeg, subtitle parsing) is shared.

### 7.4 CLI Integration

A new subcommand:

```
# Profile mode (explicit expectations)
langkit check [path] --audio-langs ja,en --sub-langs ja,en \
    --duration-tolerance 2 --subtitle-threshold 80
langkit check [path] --profile "Thai anime"

# Auto mode (consistency detection)
langkit check [path] --auto
langkit check [path] --auto --quorum 80

# Both together
langkit check [path] --profile "Thai anime" --auto

# Fail on warnings too (CI usage, strict mode)
langkit check [path] --auto --fail-on warning
```

- `--profile` loads a saved profile by name.
- `--auto` enables consistency detection. Can be used alone or
  combined with `--profile`.
- `--quorum N` sets the consensus threshold (default: 75%).
  Only meaningful with `--auto`.
- If neither `--audio-langs` / `--sub-langs` / `--profile` nor
  `--auto` are specified, only structural checks (integrity,
  duration consistency within each file) are performed.
- Output is a human-readable summary to stdout, with optional
  `--json` for machine-readable output.
- Exit code: 0 if no errors, 1 if errors were found. Warnings
  alone do not cause a non-zero exit. Override with
  `--fail-on warning` to also fail on warnings.

Profile management subcommands:

```
langkit check profiles list
langkit check profiles save "Thai anime" --audio-langs ja,th \
    --sub-langs ja,th,en
langkit check profiles delete "Thai anime"
```

### 7.5 GUI Integration

#### 7.5.1 WebRPC Service

New RIDL schema `expectation.ridl` in `api/schemas/services/`:

```ridl
webrpc = v1
name = langkit-expectation
version = v1.0.0

struct ExpectationProfile
  - name: string
  - expectedAudioLangs: []string
  - expectedSubtitleLangs: []string
  - requireVideoTrack: bool
  - requireLanguageTags: bool
  - durationTolerancePercent: float64
  - subtitleLineThresholdPct: float64
  - checkExternalAudioFiles: bool
  - videoExtensions: []string

struct AutoCheckConfig
  - enabled: bool
  - quorumPct: float64
  - softFloorPct: float64
  - minGroupSize: int32

struct ValidationIssue
  - severity: string
  - source: string
  - filePath: string
  - category: string
  - message: string

struct DirectoryConsensusSummary
  - directory: string
  - fileCount: int32
  - consensusAudioLangs: []string
  - consensusSubLangs: []string
  - consensusAudioTrackCount: int32
  - medianDurationSec: float64

struct FileSummary
  - filePath: string
  - fileName: string
  - errorCount: int32
  - warningCount: int32
  - passed: bool

struct ValidationReport
  - rootPath: string
  - totalFiles: int32
  - issues: []ValidationIssue
  - fileSummaries: []FileSummary
  - interpretedSummaries: []string
  - consensusSummaries: []DirectoryConsensusSummary
  - errorCount: int32
  - warningCount: int32
  - infoCount: int32

service ExpectationService
  - RunCheck(path: string, profile?: ExpectationProfile,
      autoConfig?: AutoCheckConfig)
      => (report: ValidationReport)
  - ListProfiles() => (profiles: []ExpectationProfile)
  - SaveProfile(profile: ExpectationProfile) => ()
  - DeleteProfile(name: string) => ()
```

The service implementation lives in `internal/api/services/`,
following the existing pattern: it implements the `api.Service`
interface (`Name()`, `Handler()`, `Description()`) and is
registered via `server.RegisterService()`.

#### 7.5.2 Frontend Feature Card & App Integration

The Expectation Checker is exposed as a **feature card** in the
`FeatureSelector`, following the existing `FeatureDefinition`
interface from `featureModel.ts`:

```typescript
{
    id: 'expectationCheck',
    label: 'Check Media',
    options: {
        mode: {
            type: 'dropdown',
            label: 'Check Mode',
            default: 'auto',
            choices: [
                { value: 'auto', label: 'Auto (detect anomalies)' },
                { value: 'profile', label: 'Profile (explicit expectations)' },
                { value: 'both', label: 'Auto + Profile' }
            ]
        },
        profile: {
            type: 'dropdown',
            label: 'Expectation Profile',
            default: '',
            choices: [],  // Dynamically populated from backend
            visibleWhen: { mode: ['profile', 'both'] }
        },
        quorum: {
            type: 'number',
            label: 'Consensus threshold (%)',
            default: 75,
            min: 50,
            max: 100,
            visibleWhen: { mode: ['auto', 'both'] }
        }
    },
    requiresLanguage: false
}
```

Auto mode is the default because it requires zero configuration --
the user just selects "Check Media", points at a directory, and
gets a report. Profile mode is available for users who want to
enforce specific expectations beyond internal consistency.

**App-level wiring** (changes to `App.svelte`):

The `expectationCheck` feature requires branching in the process
handler. Currently `App.svelte` has a single process path that
always calls `ProcessingService.SendProcessingRequest()` (~line
673). The checker needs:

1. **Process handler branching**: In `handleProcess()`, check if
   `selectedFeatures.expectationCheck` is the only selected
   feature. If so, call `ExpectationService.RunCheck()` instead
   of `SendProcessingRequest()`. This is a synchronous
   request-response (not async status-polling), so it does not
   use the existing `ProcessingStatus` mechanism.

2. **`selectedFeatures` expansion**: Add `expectationCheck: false`
   to the `selectedFeatures` object in `FeatureSelector.svelte`
   (~line 55) and wire it into the `visibleFeatures` derivation
   (~line 1817).

3. **Check-result store**: A new Svelte writable store
   (`checkResultStore`) holds the most recent `ValidationReport`.
   Set on check completion, cleared on media path change or new
   check start. This store drives the results panel and the
   confirmation handshake (see section 8).

4. **Report container**: A dedicated results panel component
   rendered below the feature cards in `App.svelte` when
   `$checkResultStore` is non-null. This does not exist in the
   current layout -- it is a new DOM region between the feature
   cards and the process button area.

#### 7.5.3 Profile Management UI

**Profile lifecycle:**

Profiles are managed within the feature card's options area:

1. **Selection**: A `Dropdown` lists saved profiles. The first
   entry is an auto-generated **"From Settings"** profile derived
   from current Settings values:
   - `expectedAudioLangs = [targetLanguage]`
   - `expectedSubtitleLangs = [targetLanguage] + nativeLanguages`
   This matches Langkit's target/reference language model: you
   expect audio in your *target* language (the one you're
   learning), and subtitles in both target and native/reference
   languages. Expecting native-language *audio* by default would
   produce false errors on libraries that (correctly) only have
   target-language audio.
2. **Creation**: A "+ New" option in the dropdown opens inline
   fields (language tag inputs, toggle switches for booleans,
   numeric inputs for thresholds). Reuses existing `TextInput`,
   `NumericInput`, and `SelectInput` components.
3. **Saving**: A "Save" button calls
   `ExpectationService.SaveProfile()`. Profiles are immediately
   available in the dropdown.
4. **Editing**: Selecting a saved profile populates the inline
   fields. Changes are saved via the same "Save" button (upsert
   by name).
5. **Deletion**: A delete icon next to the dropdown calls
   `ExpectationService.DeleteProfile()` with confirmation via
   `ConfirmDialog`.

**Frontend state**: A Svelte writable store
(`expectationProfilesStore`) caches the profile list, refreshed
on mount via `ListProfiles()` and after any save/delete operation.

#### 7.5.4 Results Display

The report UI has three tiers, so it scales from "quick glance"
to "full detail" without overwhelming:

**Tier 1 -- Summary bar** (always visible after a check):

A single line with counts and overall pass/fail:
`"12 files checked: 3 errors, 2 warnings -- 9 passed, 3 failed"`

**Tier 1.5 -- Consensus overview** (auto mode only, below summary
bar):

When auto mode runs, display the inferred consensus for each
directory group: "Season 1 (24 files): audio [ja, th], subtitles
[ja, en, th], median duration 23m 45s". This gives the user
transparency into what the checker considers "normal" before
showing deviations from it.

**Tier 2 -- Interpreted summaries** (collapsible, default open if
errors exist):

Human-readable sentences derived from the raw findings.

Profile mode examples:
- "Directory X does not contain media with audiotrack in language
  Japanese."
- "Subtitle in language English is missing for S02E21."
- "Audiotrack of S01E02 in language Thai appears corrupted."
- "7 files are missing Japanese audio."

Auto mode examples:
- "S01E14 is missing Japanese audio (present in 23 of 24 files)."
- "S01E03 (12m 34s) is unusually short (median: 23m 45s)."
- "S01E07 has 3 audio tracks (most files have 2)."

These are generated server-side (see section 7.6) and delivered
as `interpretedSummaries` in the report. They aggregate common
patterns (e.g. "7 files missing JP audio" instead of 7 individual
issue lines).

**Tier 3 -- Raw findings** (collapsible, default collapsed):

Per-file expandable rows showing every individual `Issue` with
severity badge, category, and message. This is the "I want to see
everything" view.

**Filtering**: Severity filter buttons (Error / Warning / Info)
at the top of both Tier 2 and Tier 3. Info is hidden by default.
When both modes run together, an additional source filter
(Profile / Auto) lets the user focus on one type of finding.

**Lite mode**:

The results component respects `$liteModeStore.enabled`:
- No slide/fade transitions on collapsible panels.
- Static color indicators instead of animated severity badges.
- No backdrop-blur, complex shadows, or gradient animations.
- Standard HTML elements throughout.
- Follows the same conditional pattern as `GlowEffect.svelte`,
  `LogViewer.svelte`, etc.

**Design alignment**:

- Reuses existing design tokens and component library (`TextInput`,
  `SelectInput`, `Hovertip`, `Dropdown`, `ConfirmDialog`).
- Severity badges follow `ProcessErrorTooltip` / `ErrorCard`
  styling.
- Profile editing reuses the inline-edit pattern from
  `Settings.svelte`.

### 7.6 Interpreted Summaries

The report contains both raw `Issues` (one per finding per file)
and `interpretedSummaries` (human-readable sentences that aggregate
and contextualize the raw data). Summaries are generated
server-side in `expectation_report.go`.

**Aggregation rules (profile mode):**

- If N files share the same issue (e.g. missing audio in language
  X), emit one summary: "N files are missing [language] audio."
- If all files in a directory fail the same check, emit a
  directory-level summary: "Directory [name] does not contain
  media with audiotrack in language [X]."
- Per-file issues that are unique get individual summaries:
  "Subtitle in language [X] is missing for [filename]."
  "Audiotrack of [filename] in language [X] appears corrupted."

**Aggregation rules (auto mode):**

Auto summaries are phrased in terms of deviation from the group
norm, not deviation from user expectations:

- **Consensus header** (always shown for auto mode): "Consensus
  for [directory]: [N] files, audio: [ja, th], subtitles:
  [ja, en, th], median duration: 23m 45s."
  This tells the user what the checker considers "normal".
- **Language anomalies**: "S01E14 is missing Japanese audio
  (present in 23 of 24 files)."
  "S02E03 and S02E07 are missing English subtitles (present in
  10 of 12 files)."
- **Duration outliers**: "S01E03 (12m 34s) is unusually short
  compared to siblings (median: 23m 45s)."
- **Track count anomalies**: "S01E07 has 3 audio tracks (most
  files have 2)."

**Naming**: Filenames in summaries use the base name without
extension, and attempt to extract episode identifiers (S01E02
patterns) when present.

**Source tagging**: When both modes run together, summaries are
grouped by source (profile findings first, then auto findings)
with a visual separator.

## 8. Check-to-Process Confirmation Handshake

The checker is advisory, but unacknowledged issues should not be
silently ignored. A lightweight state machine governs the
transition from check results to processing.

### 8.1 Check States

```
unchecked
  │
  ▼  (user runs check)
checked_clean                      ──► process freely
  │
checked_with_issues_unacknowledged ──► process blocked until acknowledged
  │
  ▼  (user confirms in dialog)
checked_with_issues_acknowledged   ──► process freely
```

- **`unchecked`**: No check has been run for the current media
  path. Processing proceeds normally (the checker is optional).
- **`checked_clean`**: Check completed with zero errors (warnings
  alone do not block). Processing proceeds freely.
- **`checked_with_errors_unacknowledged`**: Check completed with
  at least one **Error**. The user has not yet acknowledged the
  issues. If the user clicks "Process" in this state, a
  `ConfirmDialog` is shown with the error/warning counts and
  the user must explicitly confirm to proceed.
- **`checked_with_errors_acknowledged`**: The user has seen the
  errors and confirmed. Processing proceeds freely.

**Why errors only**: Auto mode will naturally produce
warning-heavy output (every anomaly is a Warning by design).
Blocking processing on warnings would be too aggressive and
would train users to click through the dialog reflexively.
Errors represent genuine problems (corrupted files, profile
violations) that warrant a confirmation step.

### 8.2 State Transitions

- Running a check: any state → `checked_clean` or
  `checked_with_errors_unacknowledged` (based on error count).
- Confirming the dialog: `unacknowledged` → `acknowledged`.
- Changing media path: any state → `unchecked`.
- Changing check mode, quorum, or profile and re-running:
  any state → result of new check.

### 8.3 Frontend Implementation

The state is derived from `checkResultStore`:

```typescript
// In a derived store or reactive block:
$: checkState =
    $checkResultStore === null
        ? 'unchecked'
        : $checkResultStore.errorCount === 0
            ? 'checked_clean'
            : $checkResultStore.acknowledged
                ? 'checked_with_errors_acknowledged'
                : 'checked_with_errors_unacknowledged';
```

The `acknowledged` flag is a client-side boolean on the store
entry, set to `true` when the user confirms the dialog.

In `handleProcess()` (App.svelte):

```
if checkState === 'checked_with_errors_unacknowledged':
    show ConfirmDialog with:
        "Check found {errorCount} errors, {warningCount} warnings.
         Proceed with processing anyway?"
    on confirm:
        set acknowledged = true
        proceed with SendProcessingRequest()
    on cancel:
        abort
else:
    proceed with SendProcessingRequest()
```

### 8.4 Backend Tagging (Optional, Recommended)

For traceability, the `ProcessRequest` struct (in
`processing.ridl`) can optionally carry:

```ridl
struct ProcessRequest
  - path: string
  - selectedFeatures: map<string,bool>
  - options: FeatureOptions
  - languageCode: string
  - audioTrackIndex?: int32
  - expectationAcknowledged?: bool
```

The backend does not block on this field -- it simply logs whether
processing was started with acknowledged or unacknowledged issues
(or without a check at all). Useful for crash reports and
debugging "why did this run fail".

## 9. Prerequisite Refactoring

These changes to existing code are needed before the checker can
be implemented:

1. **AudioTrack language parsing**: Upgrade `AudioTrack` in
   `mediainfo.go` to use `Lang` (with subtag) instead of bare
   `*iso.Language`. Apply `ParseLanguageTags()` to audio track
   language metadata the same way `TextTrack` already does (line
   219). Without this, `getIdx()` cannot do script-aware matching
   on audio tracks.

2. **Shared media discovery**: Extract the walk + filter logic
   from `routing.go:178-221` into a `DiscoverMediaFiles()`
   function that both `Routing()` and the checker call. This
   eliminates the drift risk where the checker validates a
   different file set than processing operates on.

3. **Subtitle candidate extraction**: Factor the core logic of
   `collectStandaloneCandidates()` and `collectAllCandidates()`
   (currently `*Task` methods) into standalone functions that
   accept a media file path and directory, so the checker can call
   them without constructing a `Task`. The `*Task` methods become
   thin wrappers. Additionally, the checker variant of standalone
   collection should **retain candidates with unknown language**
   as `Lang=und` instead of dropping them (current behavior at
   `cards.go:67-71`). This ensures untagged subtitle files still
   count toward presence; the missing tag is a separate finding.

4. **Duration normalization**: Add `parseMediaInfoDuration()` and
   `probeDuration()` in a new `internal/pkg/media/duration.go`,
   providing reliable float64-seconds conversion from both
   mediainfo strings and ffprobe output.

5. **Langkit-generated artifact exclusion**: The checker's probe
   pass must skip Langkit-generated files using the existing
   marker helpers from `markers.go`. This is already handled for
   subtitles (dubtitles via `isLangkitMadeDubtitles()`, translit
   via `isLangkitMadeTranslit()`) and media discovery (merged
   outputs via `isLangkitMadeMergedOutput()`, `.media` dirs).
   The gap is **external audio checks** (`checkExternalAudioFiles`
   in profile mode): these must also exclude files matching
   `langkitMadeVocalsOnlyMarker()` (`.VOCALS.*`) and
   `langkitMadeEnhancedMarker()` (`.VOICES.ENHANCED`). Without
   this, Langkit's own voice-separated outputs would be treated
   as user-provided external audio and generate false findings.

## 10. Performance Considerations

- **Mediainfo calls are fast** (~50-100ms per file). For a 50-file
  batch, this adds ~5 seconds.
- **FFmpeg integrity checks** are also fast for header-only probing
  (the existing `-t 0` approach). ~100-200ms per file.
- **Subtitle parsing** via `subs.OpenFile()` is fast for typical
  subtitle sizes.
- **Auto mode adds negligible overhead**: The probe pass (mediainfo,
  ffmpeg, subtitle collection) is identical to profile mode. The
  consensus computation is pure in-memory arithmetic over the
  already-collected data -- effectively free.
- For very large libraries (hundreds of files), the check should
  report progress via the existing `IncrementProgress()` mechanism
  with a dedicated progress bar ID.
- All I/O operations should respect `context.Context` for
  cancellation.

## 11. Future Development

These features are **out of scope** for the initial release but
inform the architecture -- the `FileCheckResult` / `IsExpected`
data model is designed to support them without major refactoring.

### 11.1 Library Grooming

Inspired by the existing `~/go/src/unmuxAudio/unmuxAudio.sh`
script, grooming would:

1. Identify audio tracks and subtitle files that are **not part
   of the user's expectations** (i.e. `IsExpected == false` in
   the check results).
2. Demux unwanted audio tracks from the container.
3. Move unwanted audio files and subtitle files to a user-
   specified archive directory, keeping the working directory
   clean.

This is the inverse of the expectation check: instead of "warn
me about what's missing", it's "clean up what I don't need".

The grooming feature would reuse
`ValidationReport.FileMetadata` from a preceding check run, so
the user flow is: Check -> Review -> Groom.

### 11.2 Subtitle-Audio Alignment Verification

A future enhancement that would use ASR on a small number of
samples to verify that subtitle timing aligns with the audio
track. Implementation: lightweight STT on 3-5 randomly sampled
segments, fuzzy-match transcribed text against subtitle text at
those timestamps.

### 11.3 Baseline Integration

Eventually the checker could be called automatically at the start
of `Routing()` (after `DiscoverMediaFiles()`, before the per-file
loop). Auto mode is especially suited for this: it requires no
user configuration, so it can run transparently on every batch.
Combined with a user's default profile (if one is set), this
would catch both internal inconsistencies and expectation
violations before any processing begins. The preflight-service
design makes this a single function call addition.

## 12. Open Questions

1. **External audio file discovery**: What naming convention
   should be used to associate external audio files with videos?
   The prefix-matching from `collectStandaloneCandidates()` could
   be reused.

2. **Subtitle line count threshold calibration**: Real-world
   variation may produce false positives despite subtype grouping.
   The threshold is per-profile so users can tune it.

3. **Autosub interaction**: Should the checker warn about
   suboptimal auto-selection outcomes (e.g. "only a signs-only
   track exists for language X")?

4. **Auto mode and mixed-content directories**: Mitigated by
   bonus content exclusion heuristics (section 5.3.3). The
   word-boundary matching for short tokens should prevent false
   exclusions, but may need tuning based on real-world naming
   conventions across different media sources.

5. **Auto mode for single-directory vs recursive**: When the
   user points at a parent directory (e.g. `/Anime/Series/`),
   each subdirectory forms its own group. But what about files
   directly in the root alongside subdirectories? These orphan
   files don't form a meaningful group. Current plan: only
   files in directories with ≥ `minGroupSize` media files get
   auto-checked.

## 13. Milestones

### Phase 1: Foundations & Core Checks (Profile Mode)
- Prerequisite refactoring (AudioTrack lang, shared discovery,
  candidate extraction with unknown-lang retention, duration
  normalization, Langkit artifact marker filtering)
- `ExpectationProfile`, `ValidationReport`, `Issue` types
- Video integrity check (batch) via shared discovery
- Audio/subtitle language existence validation
- CLI `langkit check` subcommand with human-readable output
  and `--fail-on` flag
- Validate signal quality: run against real media libraries,
  tune thresholds, confirm low false-positive rate

### Phase 2: Auto Mode (Consistency Detection)
- `AutoCheckConfig`, `DirectoryConsensus` types
- Confidence-tiered consensus (quorum → Warning, soft floor →
  Info, below floor → suppressed)
- Bonus content exclusion with word-boundary-aware matching
- Consistency checks 6.5.1-6.5.5 (language consensus, duration
  outliers with n≥6 gating, track count anomalies with mode-tie
  handling)
- Directory grouping for recursive scans, skipped-group info
- CLI `--auto`, `--quorum`, `--soft-floor` flags
- Auto-specific interpreted summaries (consensus header,
  deviation phrasing)
- Validate on real libraries: confirm quorum/softFloor
  thresholds and Tukey fences produce useful signal with low
  noise

### Phase 3: Subtitle Validation & Interpreted Summaries
- Subtitle integrity (parse, line count comparison with subtype
  awareness)
- Duration consistency checks with hybrid tolerance (profile)
- Full interpreted summary generation for both modes
  (aggregation, episode-aware naming, source tagging)
- `--json` output for CLI

### Phase 4: Profile Persistence
- `config/profiles.go` (YAML marshal/unmarshal with wrapper)
- CLI `langkit check profiles` subcommands
- WebRPC profile CRUD endpoints

### Phase 5: GUI Integration
- WebRPC `ExpectationService` with `RunCheck()`
- Feature card with mode selector (Auto / Profile / Both),
  profile dropdown, quorum slider, "From Settings" default
- Profile management UI (create, edit, delete)
- Three-tier results display with consensus overview
  (summary bar, consensus header, interpreted summaries,
  raw findings)
- Source and severity filtering, lite mode support
- Error-driven confirmation handshake (warnings do not block)
- `checkResultStore`, App.svelte process handler branching
- Progress reporting during check
