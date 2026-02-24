# PRD: Decode-Depth Integrity Checks (Routing + Expectation Checker)

## 1. Context

Langkit currently uses a fast FFmpeg header/container integrity probe
(`media.CheckValidData()` with `-t 0`) in `routing.go:checkIntegrity()`.

This misses real decode-time corruption in specific streams. A confirmed
example is an embedded Thai AAC track that fails decode while header
checks pass.

Expectation Checker now exists as an optional preflight feature with
`auto` and `profile` modes. Integrity behavior must be coherent across:
- default processing flow (routing),
- expectation checker (`auto` and `profile`).

## 2. Problem

Header-only integrity is too weak for modern multi-track media:
- container/header can be valid,
- one embedded audio stream can still be bitstream-corrupted.

Result: users discover failures late (or get silent bad outcomes) after
starting long processing runs.

## 3. Goal

Replace header-only integrity logic with decode-based integrity logic,
with one shared policy controlling depth:
- `sampled` (default, fast)
- `full` (strict)

This policy must apply to expectation checker in both `auto` and
`profile` modes via a new global setting (not a per-profile checkbox).

## 4. Non-Goals

- Auto-repairing corrupted streams.
- Adding many tunables in v1 (keep configuration minimal).
- Full end-to-end video decode in v1 (cost too high for bulk defaults).

## 5. Evidence Collected

Benchmark on a 24-episode folder (SSD, modern CPU):

- `header_only_t0`: avg `0.126s/file`
- `full_audio_all_streams`: avg `0.877s/file`
- `full_audio_lang_tha`: avg `0.879s/file`
- `sample_pre_audio_lang_tha_start`: avg `0.072s/file`
- `sample_pre_audio_lang_tha_mid`: avg `0.073s/file`
- `sample_pre_audio_lang_tha_end`: avg `0.063s/file`
- `sample_pre_video_start`: avg `0.577s/file`
- `sample_pre_video_mid`: avg `0.531s/file`
- `sample_pre_video_end`: avg `0.640s/file`

Key conclusions:
1. Full decode is substantially costlier than header checks.
2. Sampled decode with `-ss` **before** input is very fast.
3. `-ss` before input is much faster than `-ss` after input for sampled
   checks on this dataset.
4. Mid/end samples catch corruption cases that start-sample misses.
5. Sampled video probing is materially cheaper than full decode and is
   acceptable as a default bulk integrity signal.

## 6. Product Decision

### 6.1 Integrity depth policy

Introduce a shared integrity depth enum:
- `sampled` (default)
- `full`

### 6.2 Where policy applies

- Routing pre-check (`checkIntegrity` path).
- Expectation Checker `profile` mode.
- Expectation Checker `auto` mode.

### 6.3 Stream scope by context

- **Routing** (default processing path):
  - `sampled`: decode sampled windows on target/relevant audio stream
    plus sampled video windows (fallback rule defined below).
  - `full`: full decode of selected audio scope plus sampled video
    windows (video does not switch to full decode).

- **Expectation checker / profile mode**:
  - audio scope derived from `expectedAudioLangs`.

- **Expectation checker / auto mode**:
  - audio scope derived from quorum/consensus audio languages.

### 6.4 Fallback rules

If desired language stream is not found:
- fallback to first audio stream for routing,
- fallback to all audio streams for expectation checker,
- never silently skip integrity checks.

## 7. Settings / UX

Add one global setting in `Settings`:
- `integrityDecodeDepth`: `sampled | full`
- default: `sampled`

This setting governs integrity depth for both expectation checker modes
(`auto`, `profile`) and routing integrity checks.

UI in Settings panel:
- label: `Integrity decode depth`
- options:
  - `Sampled (Recommended)`
  - `Full (Strict, slower)`
- help text:
  - Sampled checks selected audio/video segments (fast).
  - Full decodes selected audio streams end-to-end (strict); video
    remains sampled.

No per-profile checkbox for this.

## 8. Technical Design

### 8.1 New media integrity API

Add decode-based integrity helpers in media package (new file preferred,
e.g. `internal/pkg/media/integrity_decode.go`):

```go
type IntegrityDepth string
const (
    IntegritySampled IntegrityDepth = "sampled"
    IntegrityFull    IntegrityDepth = "full"
)

type DecodeScope struct {
    AudioStreamIndices []int
    CheckVideoSamples  bool
}

type DecodeCheckOptions struct {
    Depth       IntegrityDepth
    SampleSec   int      // default 20
    SamplePoints []string // default ["start", "mid", "end"]
    Scope       DecodeScope
}

func CheckDecodeIntegrity(path string, opt DecodeCheckOptions) (isCorrupted bool, err error)
```

Implementation intent (not prescriptive about exact command assembly):
- perform decode-level validation, not header/container-only checks.
- treat decoder failures as corruption (`isCorrupted=true`).
- keep sampled mode fast enough for bulk usage.
- use explicit stream indices for stable targeting.

### 8.2 Stream mapping source

Do not rely on fragile `-map 0:m:language:...` directly.
Resolve stream indices from existing metadata flow (`Mediainfo`) and map
by explicit index (`-map 0:<index>`).

### 8.3 Video in v1

Video-sampled probing is enabled by default in v1 using the same sample
points as sampled audio checks.

Depth behavior:
- `sampled`: sampled audio + sampled video.
- `full`: full audio decode + sampled video.

Full video decode is explicitly out of v1 scope due high bulk-run cost.

### 8.4 FFmpeg command reference (non-normative)

The following command shapes are reference examples only. Equivalent
commands/flags are acceptable as long as behavior matches this PRD.

Resolve audio stream indices first:

```bash
ffprobe -v error -select_streams a \
  -show_entries stream=index:stream_tags=language \
  -of csv=p=0 "$FILE"
```

Sampled decode for selected audio indices:

```bash
ffmpeg -hide_banner -v error -xerror -err_detect explode+crccheck \
  -ss "$SAMPLE_TS" -i "$FILE" -t "$SAMPLE_SEC" \
  -map 0:$AUDIO_INDEX_1 -map 0:$AUDIO_INDEX_2 \
  -vn -sn -dn -f null -
```

Sampled decode for video:

```bash
ffmpeg -hide_banner -v error -xerror -err_detect explode+crccheck \
  -ss "$SAMPLE_TS" -i "$FILE" -t "$SAMPLE_SEC" \
  -map 0:v:0 -an -sn -dn -f null -
```

Full decode for selected audio indices (`full` depth):

```bash
ffmpeg -hide_banner -v error -xerror -err_detect explode+crccheck \
  -i "$FILE" \
  -map 0:$AUDIO_INDEX_1 -map 0:$AUDIO_INDEX_2 \
  -vn -sn -dn -f null -
```

Notes:
- sampled mode should use `-ss` before `-i` for performance in bulk.
- full depth applies to audio streams only in v1; video remains sampled.

## 9. Integration Plan

### 9.1 Routing integration

Replace `media.CheckValidData()` call in `routing.go:checkIntegrity()`
with decode-depth path:
1. resolve relevant audio scope from task/language/mediainfo,
2. run `CheckDecodeIntegrity` with global depth setting,
3. preserve existing abort semantics (`AbortTask` behavior).

### 9.2 Expectation checker integration

In checker probe flow:
- use shared decode-integrity function,
- select scope per mode:
  - profile: expected audio languages,
  - auto: quorum/consensus languages,
- if mode scope is empty, fallback to all audio streams.

Issue semantics:
- decode errors are structural integrity errors.
- expectation checker report includes stream/language context in message.

## 10. CLI / API implications

### 10.1 Settings

Add `integrityDecodeDepth` to:
- `internal/config/settings.go`
- settings persistence defaults (`sampled`)
- frontend settings store/types.

### 10.2 Expectation checker API

No new profile field required for depth.
Depth comes from global settings (or optional request override in future
phase if needed).

## 11. Behavior Matrix

| Context | Depth | Audio scope |
|---|---|---|
| Routing | sampled | target/relevant stream, fallback first audio |
| Routing | full | selected stream(s), fallback first/all audio |
| Checker profile | sampled | expected audio langs |
| Checker profile | full | expected audio langs |
| Checker auto | sampled | consensus/quorum langs |
| Checker auto | full | consensus/quorum langs |

If scope empty: checker falls back to all audio streams.

Video behavior (all contexts):
- sampled video windows are checked in both `sampled` and `full`.
- `full` does not escalate video to full decode.

## 12. Migration and Backward Compatibility

- Existing header-only integrity path is replaced in active flow.
- No profile schema expansion needed for depth.
- Existing expectation checker modes remain unchanged (`auto/profile/both`).
- New global setting defaults to `sampled`, preserving fast UX while
  increasing corruption detection compared to header-only.

## 13. Implementation Phases

### Phase A (must-have)
1. Add `integrityDecodeDepth` setting + UI.
2. Implement shared decode integrity helper (audio scope + sampled video).
3. Replace routing `checkIntegrity()` with shared helper.
4. Wire expectation checker auto/profile to shared helper scope rules.
5. Preserve existing error handling behavior (`AbortTask` etc.).

## 14. Acceptance Criteria

1. A file with corrupted embedded audio stream fails integrity in
   routing and expectation checker under both `sampled` and `full`
   (given corruption occurs in sampled windows for sampled mode).
2. Default (`sampled`) is materially faster than `full` on bulk runs.
3. Expectation checker `auto` uses consensus-derived scope; `profile`
   uses expected-audio scope.
4. No “skip silently” behavior when language mapping is missing.
5. UX exposes one global depth setting controlling both checker modes.
6. Sampled video probes run by default in both routing and expectation
   checker paths.
7. `full` depth escalates only audio scope to full decode; video stays
   sampled.
