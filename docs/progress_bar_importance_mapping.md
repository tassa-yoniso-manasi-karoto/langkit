# Progress Bar Importance Mapping

## Purpose

This document serves as the **ground truth** for progress bar visual importance based on user-selected features:

1. **Ground truth for the test suite** - Validates that the importance allocation algorithm produces correct results for all feature combinations
2. **Human-readable reference** - A visualization for anyone interested in understanding how progress bar hierarchy works
3. **Easy to reason with** - Emoji-based abstraction allows height classes to be adjusted independently

## How to Read This Document

- Each table row represents a valid feature combination (e.g., `subs2cards+enhance+translit`)
- Each column represents a progress bar ID (e.g., `demucs-process`, `item-bar`)
- Cell values are importance level emojis (🟥🟧🟨🟩🟦) or `-` if the bar doesn't appear for that combination
- HTML comments above each table (e.g., `<!-- MODE: subs2cards, BULK: false -->`) provide parsable metadata

## Importance Levels

| Emoji | Level | Description |
|-------|-------|-------------|
| 🟥 | Very Important | Primary feature, maximum visual prominence |
| 🟧 | Important | Secondary processing or primary downloads |
| 🟨 | Normal | Standard visibility |
| 🟩 | Low Importance | Reduced visibility |
| 🟦 | Very Low Importance | Minimal visual presence |

## Progress Bar Types

### Processing Bars
| ID | Feature | Description |
|----|---------|-------------|
| `media-bar` | Bulk Processing | Counts video files processed (only in bulk mode) |
| `item-bar` | Subs2Cards / Subs2Dubs / Condense | Counts subtitle lines processed |
| `demucs-process` | Voice Enhancement (Demucs) | Voice separation processing |
| `audiosep-process` | Voice Enhancement (MelBand RoFormer) | Voice separation processing (alternative to Demucs) |
| `translit-process` | Transliteration | Romanization/tokenization processing |

### Download/Setup Bars
| ID | Feature | Description |
|----|---------|-------------|
| `demucs-docker-dl` | Voice Enhancement (Demucs) | Docker image download (first-time) |
| `demucs-model-dl` | Voice Enhancement (Demucs) | Model weights download |
| `audiosep-docker-dl` | Voice Enhancement (MelBand RoFormer) | Docker image download (first-time) |
| `audiosep-model-dl` | Voice Enhancement (MelBand RoFormer) | Model weights download |
| `translit-docker-dl` | Transliteration | Provider Docker image download |
| `translit-init` | Transliteration | Database initialization (e.g., Ichiran) |

## Sequential Bar Rules

The following bars occur sequentially (not concurrently) and can share the same importance level:

1. **item-bar** and **translit-process**: Subtitle processing happens before/after transliteration
2. **translit-docker-dl** and **translit-init**: Both are setup phases, happen in sequence
3. **demucs-docker-dl** and **demucs-model-dl**: Both are setup phases, happen in sequence
4. **audiosep-docker-dl** and **audiosep-model-dl**: Both are setup phases, happen in sequence

## Voice Enhancement Providers

Demucs and Audio-Separator (MelBand RoFormer) are **alternative providers** for the same Enhance mode. Only one is active at a time based on user selection. Both get **identical importance levels** since they serve the same purpose.

## Core Principles

1. **Mode determines primary feature** - Gets highest available importance
2. **Downloads are ONE level below their parent feature's processing**
3. **Processing bars never go below 🟨 (Normal)** - Only downloads can be 🟩/🟦
4. **Bulk mode shifts everything down by one level** with media-bar taking 🟥

---

# PART 1: Single File Mode

No `media-bar`. Primary feature gets maximum visual importance.

## 1.1 ENHANCE Mode

<!-- MODE: enhance, BULK: false -->
| Combination | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| enhance | 🟧 | 🟨 | 🟥 | 🟧 | 🟨 | 🟥 |

## 1.2 TRANSLIT Mode

<!-- MODE: translit, BULK: false -->
| Combination | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process | item-bar |
|-------------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|----------|
| translit | 🟧 | 🟧 | 🟥 | - | - | - | - | - | - | - |
| translit+enhance | 🟨 | 🟨 | 🟥 | 🟨 | 🟩 | 🟥 | 🟨 | 🟩 | 🟥 | - |
| translit+condense | 🟧 | 🟧 | 🟥 | - | - | - | - | - | - | 🟥 |
| translit+enhance+condense | 🟨 | 🟨 | 🟥 | 🟨 | 🟩 | 🟥 | 🟨 | 🟩 | 🟥 | 🟥 |

## 1.3 CONDENSE Mode

<!-- MODE: condense, BULK: false -->
| Combination | item-bar | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|----------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| condense | 🟥 | - | - | - | - | - | - |
| condense+enhance | 🟥 | 🟨 | 🟩 | 🟥 | 🟨 | 🟩 | 🟥 |

## 1.4 SUBS2DUBS Mode

<!-- MODE: subs2dubs, BULK: false -->
| Combination | item-bar | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|----------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| subs2dubs | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2dubs+enhance | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2dubs+translit | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2dubs+condense | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2dubs+enhance+translit | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2dubs+enhance+condense | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2dubs+translit+condense | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2dubs+enhance+translit+condense | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |

## 1.5 SUBS2CARDS Mode

<!-- MODE: subs2cards, BULK: false -->
| Combination | item-bar | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|----------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| subs2cards | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+translit | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2cards+condense | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2cards+stt | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance+translit | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+enhance+condense | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+enhance+stt | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+translit+condense | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2cards+translit+stt | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2cards+condense+stt | 🟥 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance+translit+condense | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+enhance+translit+stt | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+enhance+condense+stt | 🟥 | - | - | - | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |
| subs2cards+translit+condense+stt | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - |
| subs2cards+enhance+translit+condense+stt | 🟥 | 🟨 | 🟨 | 🟧 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |

---

# PART 2: Bulk Mode

`media-bar` takes top priority. All other bars shift down by one importance level.

## 2.1 ENHANCE Mode (Bulk)

<!-- MODE: enhance, BULK: true -->
| Combination | media-bar | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|-----------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| enhance | 🟥 | 🟨 | 🟩 | 🟧 | 🟨 | 🟩 | 🟧 |

## 2.2 TRANSLIT Mode (Bulk)

<!-- MODE: translit, BULK: true -->
| Combination | media-bar | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process | item-bar |
|-------------|-----------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|----------|
| translit | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - | - |
| translit+enhance | 🟥 | 🟩 | 🟩 | 🟧 | 🟩 | 🟦 | 🟧 | 🟩 | 🟦 | 🟧 | - |
| translit+condense | 🟥 | 🟨 | 🟨 | 🟧 | - | - | - | - | - | - | 🟧 |
| translit+enhance+condense | 🟥 | 🟩 | 🟩 | 🟧 | 🟩 | 🟦 | 🟧 | 🟩 | 🟦 | 🟧 | 🟧 |

## 2.3 CONDENSE Mode (Bulk)

<!-- MODE: condense, BULK: true -->
| Combination | media-bar | item-bar | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|-----------|----------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| condense | 🟥 | 🟧 | - | - | - | - | - | - |
| condense+enhance | 🟥 | 🟧 | 🟩 | 🟦 | 🟧 | 🟩 | 🟦 | 🟧 |

## 2.4 SUBS2DUBS Mode (Bulk)

<!-- MODE: subs2dubs, BULK: true -->
| Combination | media-bar | item-bar | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|-----------|----------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| subs2dubs | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2dubs+enhance | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2dubs+translit | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2dubs+condense | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2dubs+enhance+translit | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2dubs+enhance+condense | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2dubs+translit+condense | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2dubs+enhance+translit+condense | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |

## 2.5 SUBS2CARDS Mode (Bulk)

<!-- MODE: subs2cards, BULK: true -->
| Combination | media-bar | item-bar | translit-docker-dl | translit-init | translit-process | demucs-docker-dl | demucs-model-dl | demucs-process | audiosep-docker-dl | audiosep-model-dl | audiosep-process |
|-------------|-----------|----------|-------------------|---------------|------------------|------------------|-----------------|----------------|--------------------| ------------------|------------------|
| subs2cards | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+translit | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2cards+condense | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2cards+stt | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance+translit | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+enhance+condense | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+enhance+stt | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+translit+condense | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2cards+translit+stt | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2cards+condense+stt | 🟥 | 🟧 | - | - | - | - | - | - | - | - | - |
| subs2cards+enhance+translit+condense | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+enhance+translit+stt | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+enhance+condense+stt | 🟥 | 🟧 | - | - | - | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |
| subs2cards+translit+condense+stt | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | - | - | - | - | - | - |
| subs2cards+enhance+translit+condense+stt | 🟥 | 🟧 | 🟩 | 🟩 | 🟨 | 🟩 | 🟦 | 🟨 | 🟩 | 🟦 | 🟨 |

---

# Algorithm Specification

This section provides complete details for implementing the progress bar importance allocation algorithm.

## Context: Task Modes

The **task mode** represents the least significant feature that a user has chosen for a given feature combination, where that feature is the default behavior and non-negotiable. Task modes are defined in `internal/core/task.go`:

```go
const (
    Subs2Cards = iota  // Anki note creation is primary
    Subs2Dubs          // Dubtitle generation via STT is primary
    Enhance            // Voice separation/enhancement is primary
    Translit           // Subtitle romanization/tokenization is primary
    Condense           // Condensed audio creation is primary
)
```

The mode is determined by `handler.go` based on the processing request from the frontend.

## Context: Bulk Processing

When `IsBulkProcess = true` (processing a directory with multiple files), the `media-bar` appears to track total files processed. This shifts all other progress bars down by one importance level.

## Algorithm: Single File Mode (IsBulkProcess = false)

When processing a single file, the primary feature (determined by Mode) gets maximum visual importance:

| Priority | What | Importance |
|----------|------|------------|
| 1 | Primary feature processing (from Mode) | 🟥 Very Important |
| 2 | Primary feature downloads | 🟧 Important |
| 3 | Secondary feature processing | 🟧 Important |
| 4 | Secondary feature downloads | 🟨 Normal |

## Algorithm: Bulk Mode (IsBulkProcess = true)

When processing multiple files, `media-bar` takes top priority and everything shifts down:

| Priority | What | Importance |
|----------|------|------------|
| 1 | `media-bar` (file counter) | 🟥 Very Important |
| 2 | Primary feature processing | 🟧 Important |
| 3 | Primary feature downloads | 🟨 Normal |
| 4 | Secondary feature processing | 🟨 Normal |
| 5 | Secondary feature downloads | 🟩 Low Importance |

## Rule: Downloads Are One Level Below Parent Processing

A download/setup progress bar is always **ONE importance level below** its parent feature's processing bar.

Example: If `demucs-process` is 🟧, then `demucs-docker-dl` is 🟨.

## Rule: Processing Bars Never Go Below Normal (🟨)

Processing bars represent actual work being done. They should never drop below 🟨 (Normal) importance, even in complex multi-feature bulk scenarios. Only download/setup bars can be 🟩 or 🟦.

## Rule: Sequential Bars Can Share Importance

The following progress bars occur **sequentially** (not concurrently) and therefore can share the same importance level without visual conflict:

1. **`item-bar`** and **`translit-process`**: Subtitle line processing and transliteration happen in sequence
2. **`translit-docker-dl`** and **`translit-init`**: Both are setup phases that happen one after another
3. **`demucs-docker-dl`** and **`demucs-model-dl`**: Both are setup phases, but note the sub-download rule below
4. **`audiosep-docker-dl`** and **`audiosep-model-dl`**: Both are setup phases (same as demucs)

## Rule: Model Download Is a Sub-Download

`*-model-dl` bars (model weights download) are **sub-steps** of the Docker setup. They are always **ONE level below** their corresponding `*-docker-dl`:

| *-docker-dl | *-model-dl |
|-------------|------------|
| 🟧 | 🟨 |
| 🟨 | 🟩 |
| 🟩 | 🟦 |

This applies to both `demucs-model-dl` and `audiosep-model-dl`.

## Rule: All Features Enabled

When a user enables ALL features (the most complex scenario), secondary feature processing bars stay at 🟨 (Normal), not lower. The user explicitly chose all these features, so they all matter.

## Implementation Approach

When `handler.go` receives a processing request from the frontend:

1. Determine the task mode based on enabled features
2. Determine if bulk processing (`IsBulkProcess`)
3. Pre-allocate importance levels for each progress bar ID based on the rules above
4. Store these in a map keyed by progress bar ID
5. `IncrementProgress` calls can pass empty string for height class to use the pre-allocated value, or pass an explicit value to override

## Primary Feature by Mode

| Mode | Primary Processing Bars | Primary Download Bars |
|------|------------------------|----------------------|
| Enhance | `demucs-process`, `audiosep-process` | `demucs-docker-dl`, `demucs-model-dl`, `audiosep-docker-dl`, `audiosep-model-dl` |
| Translit | `translit-process` | `translit-docker-dl`, `translit-init` |
| Condense | `item-bar` | (none) |
| Subs2Dubs | `item-bar` | (none) |
| Subs2Cards | `item-bar` | (none) |

Note: Demucs and Audio-Separator are alternative providers for voice enhancement. Only one is active at runtime, but both are pre-allocated identical importance levels.

## Secondary Features

Any feature enabled that is NOT the primary feature (from Mode) is considered secondary. Its processing bar gets demoted by one level from primary, and its downloads get demoted by one level from that.
