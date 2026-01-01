# Adding Progress Bars to the Dynamic Importance System

This guide explains how to add new progress bars to langkit's Dynamic Progress Bar Importance Allocation system, which automatically assigns visual prominence based on task mode and feature combinations.

## Overview

The system ensures that the most important progress bar for the current operation gets maximum visual prominence (large SVG waves), while secondary features get progressively smaller bars. This adapts dynamically based on:

- **Task Mode** (Subs2Cards, Subs2Dubs, Enhance, Translit, Condense)
- **Bulk vs Single File** processing
- **Which features are enabled** together

When adding a new feature with progress tracking, you need to integrate it into this system so it displays appropriately alongside existing features.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Progress Bar System                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  internal/pkg/progress/constants.go     ← Bar ID constants (Step 1)     │
│           │                                                              │
│           ▼                                                              │
│  internal/core/progress_importance.go   ← Algorithm (Step 2)            │
│           │                                                              │
│           ▼                                                              │
│  docs/progress_bar_importance_mapping.md ← Ground truth (Step 3)        │
│           │                                                              │
│           ▼                                                              │
│  internal/core/progress_importance_test.go ← Verification               │
│           │                                                              │
│           ▼                                                              │
│  internal/core/handler.go               ← Runtime lookup                │
│           │                                                              │
│           ▼                                                              │
│  Your feature code                      ← Uses constants (Step 4)       │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Step-by-Step Guide

### Step 1: Add Bar ID Constants

Add your new bar ID(s) to `internal/pkg/progress/constants.go`:

```go
package progress

const (
    // ... existing constants ...

    // Your new feature bars
    BarMyFeatureProcess  = "myfeature-process"   // Processing progress
    BarMyFeatureDockerDL = "myfeature-docker-dl" // Docker image download
    BarMyFeatureModelDL  = "myfeature-model-dl"  // Model weights download
)
```

**Naming conventions:**
- Use lowercase with hyphens: `myfeature-process`
- Processing bars: `{feature}-process`
- Docker download bars: `{feature}-docker-dl`
- Model/weights download bars: `{feature}-model-dl`
- Initialization bars: `{feature}-init`

**Prefix matching:** The system uses prefix matching, so `myfeature-process-12345` will match `myfeature-process`. Append timestamps or unique IDs to your bar IDs if needed for multiple concurrent instances.

### Step 2: Update the Importance Algorithm

Edit `internal/core/progress_importance.go` to include your new bars in `ComputeImportanceMap()`.

#### 2a. If adding a new primary feature (new Mode)

If your feature can be the primary mode (like Enhance or Translit), add a new case:

```go
switch mode {
case Enhance:
    // ... existing ...
case Translit:
    // ... existing ...
case MyFeature:  // New mode
    m[progress.BarMyFeatureProcess] = primaryProcLevel
    m[progress.BarMyFeatureDockerDL] = primaryDLLevel
    m[progress.BarMyFeatureModelDL] = demoteLevel(primaryDLLevel, true)
case Condense, Subs2Dubs, Subs2Cards:
    m[progress.BarItemBar] = primaryProcLevel
}
```

#### 2b. If adding as a secondary feature

If your feature can be enabled alongside other modes (like how Enhance can be enabled with Subs2Cards), add a secondary feature handler:

```go
// If myfeature is enabled but not primary (mode != MyFeature)
if features.HasMyFeature && mode != MyFeature {
    m[progress.BarMyFeatureProcess] = secondaryProcLevel
    m[progress.BarMyFeatureDockerDL] = secondaryDLLevel
    m[progress.BarMyFeatureModelDL] = demoteLevel(secondaryDLLevel, true)
}
```

#### 2c. Update FeatureSet if needed

If your feature can be enabled as a secondary feature, add it to the `FeatureSet` struct:

```go
type FeatureSet struct {
    HasEnhance    bool
    HasTranslit   bool
    HasCondense   bool
    HasSTT        bool
    HasMyFeature  bool  // New field
}
```

And update `hasCompetingSecondaryFeature()` if your feature competes for visual space:

```go
func hasCompetingSecondaryFeature(mode Mode, features FeatureSet) bool {
    switch mode {
    case Enhance:
        return features.HasTranslit || features.HasMyFeature
    case Translit:
        return features.HasEnhance || features.HasMyFeature
    // ... etc
    }
}
```

#### 2d. Update handler.go

In `SendProcessingRequest()`, add your feature to the FeatureSet construction:

```go
features := FeatureSet{
    HasEnhance:    req.SelectedFeatures["voiceEnhancing"],
    HasTranslit:   req.SelectedFeatures["subtitleRomanization"] || ...,
    HasCondense:   req.SelectedFeatures["condensedAudio"],
    HasSTT:        tsk.STT != "",
    HasMyFeature:  req.SelectedFeatures["myFeature"],  // New
}
```

### Step 3: Update the Ground Truth Document

Edit `docs/progress_bar_importance_mapping.md` to add your new bar columns to all relevant tables.

#### 3a. Add to Progress Bar Types section

```markdown
### Processing Bars
| ID | Feature | Description |
|----|---------|-------------|
| `media-bar` | Bulk Processing | Counts video files processed |
| `item-bar` | Subs2Cards / Subs2Dubs | Counts subtitle lines |
| `myfeature-process` | My Feature | My feature processing |  <!-- New -->

### Download/Setup Bars
| ID | Feature | Description |
|----|---------|-------------|
| `myfeature-docker-dl` | My Feature | Docker image download |  <!-- New -->
| `myfeature-model-dl` | My Feature | Model weights download |  <!-- New -->
```

#### 3b. Add columns to all mode tables

For every table in the document, add columns for your new bars. Example for single-file mode:

```markdown
<!-- MODE: subs2cards, BULK: false -->
| Combination | item-bar | ... | myfeature-docker-dl | myfeature-model-dl | myfeature-process |
|-------------|----------|-----|---------------------|--------------------| ------------------|
| subs2cards | 🟥 | ... | - | - | - |
| subs2cards+myfeature | 🟥 | ... | 🟨 | 🟩 | 🟧 |
```

**Importance level emojis:**
| Emoji | Level | Height Class |
|-------|-------|--------------|
| 🟥 | VeryImportant | h-5 |
| 🟧 | Important | h-4 |
| 🟨 | Normal | h-3 |
| 🟩 | LowImportance | h-2 |
| 🟦 | VeryLowImportance | h-1 |
| `-` | NotApplicable | (bar doesn't appear) |

**Rules to follow:**
1. Processing bars never go below 🟨 (Normal)
2. Downloads are ONE level below their parent processing bar
3. Sub-downloads (like model-dl) are ONE level below the main download
4. Bulk mode shifts everything down by one level

### Step 4: Use the Constants in Your Code

In your feature's implementation, use the constants and pass empty string for height class:

```go
import "github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"

// For processing progress
handler.IncrementProgress(
    progress.BarMyFeatureProcess,  // Or with suffix: progress.BarMyFeatureProcess + "-" + uniqueID
    increment,
    total,
    30,                            // priority
    "Processing...",
    "description",
    "",                            // Empty string = use importance map
)

// For download progress
handler.IncrementDownloadProgress(
    progress.BarMyFeatureDockerDL,
    int(bytesIncrement),
    int(totalBytes),
    20,
    "Downloading...",
    status,
    "",                            // Empty string = use importance map
    humanize.Bytes(current) + " / " + humanize.Bytes(total),
)
```

**Important:** Always pass empty string `""` for the height class parameter. The system will look up the appropriate height class from the importance map computed at the start of processing.

### Step 5: Run Tests

Run the importance tests to verify your algorithm matches the document:

```bash
go test -v -run "TestComputeImportanceMap" ./internal/core/
```

If tests fail, the output will show exactly which combination and bar has a mismatch:

```
Line 80: subs2cards+myfeature (mode=Subs2Cards, bulk=false) bar "myfeature-process":
    expected 🟧 (h-4), got 🟨 (h-3)
```

## Complete Example: Adding Audio-Separator Support

Here's a real example of adding the audio-separator (MelBand RoFormer) progress bars:

### 1. Constants (`internal/pkg/progress/constants.go`)

```go
// Audio-separator / MelBand RoFormer bars
BarAudioSepProcess  = "audiosep-process"   // Voice separation processing
BarAudioSepDockerDL = "audiosep-docker-dl" // Docker image download
BarAudioSepModelDL  = "audiosep-model-dl"  // Model weights download
```

### 2. Algorithm (`internal/core/progress_importance.go`)

Since audio-separator is an alternative to Demucs for the Enhance mode, it shares the same importance slots:

```go
case Enhance:
    // Both demucs and audio-separator use these levels when Enhance is primary
    m[progress.BarDemucsProcess] = primaryProcLevel
    m[progress.BarDemucsDockerDL] = primaryDLLevel
    m[progress.BarDemucsModelDL] = demoteLevel(primaryDLLevel, true)
    // Audio-separator gets same levels (only one will be active at a time)
    m[progress.BarAudioSepProcess] = primaryProcLevel
    m[progress.BarAudioSepDockerDL] = primaryDLLevel
    m[progress.BarAudioSepModelDL] = demoteLevel(primaryDLLevel, true)
```

### 3. Feature Code Usage

```go
// In audio_separator_manager.go
handler.IncrementProgress(
    progress.BarAudioSepProcess,
    increment,
    100,
    30,
    "Voice Separation",
    "Processing with MelBand RoFormer...",
    "",  // Use importance map
)
```

## Debugging Tips

### Check importance map at runtime

Add temporary logging to see what importance map was computed:

```go
// In handler.go SendProcessingRequest()
h.importanceMap = ComputeImportanceMap(tsk.Mode, isBulk, features)
for barID, level := range h.importanceMap {
    h.logger.Debug().
        Str("barID", barID).
        Str("level", level.String()).
        Str("heightClass", level.HeightClass()).
        Msg("Importance map entry")
}
```

### Verify prefix matching

If your bar doesn't get the expected height class, check that the bar ID you're passing starts with the constant prefix:

```go
// Good - will match BarMyFeatureProcess
taskID := progress.BarMyFeatureProcess + "-" + timestamp

// Bad - won't match because "my" doesn't start with "myfeature"
taskID := "my-process-" + timestamp
```

### Test a specific combination

Add a focused test for your new feature:

```go
func TestComputeImportanceMap_MyFeature(t *testing.T) {
    m := ComputeImportanceMap(Subs2Cards, false, FeatureSet{
        HasMyFeature: true,
    })

    if level := m[progress.BarMyFeatureProcess]; level != Important {
        t.Errorf("myfeature-process: expected %s, got %s",
            Important.String(), level.String())
    }
}
```

## Summary Checklist

- [ ] Add bar ID constants to `internal/pkg/progress/constants.go`
- [ ] Update `ComputeImportanceMap()` in `internal/core/progress_importance.go`
- [ ] Add to `FeatureSet` struct if it's a toggleable secondary feature
- [ ] Update `hasCompetingSecondaryFeature()` if it competes for visual space
- [ ] Update `SendProcessingRequest()` in `handler.go` to build FeatureSet
- [ ] Add columns to all tables in `docs/progress_bar_importance_mapping.md`
- [ ] Use constants in your feature code with empty height class string
- [ ] Run tests: `go test -v -run "TestComputeImportanceMap" ./internal/core/`
- [ ] Build and verify: `go build ./...`
