# Changelog

All notable changes to Langkit will be documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

<!--
FORMAT REQUIREMENTS (breaking these will break the changelog parser):

- Version headers MUST be: ## [x.y.z] or ## [x.y.z-tag] - YYYY-MM-DD
  Examples: ## [1.0.0] - 2024-01-15  or  ## [1.0.0-alpha] - Unreleased
- Section headers MUST be: ### Added, ### Changed, ### Fixed, ### Deprecated,
  ### Removed, or ### Security (exactly these names, case-sensitive)
- List items MUST start with "- " (hyphen + space)
- Keep one blank line between sections
- Run `go test ./internal/changelog/...` to validate after editing

Guidelines for LLMs updating this changelog from git history:

- Only include changes relevant to end users (ordinary Anki users, not developers)
- Heed the chronology and thus do NOT list "fixes" for things that were never released: they're not fixes from the user's perspective
- Omit internal/technical changes (GPU detection internals, system diagnostics, refactoring, etc.)
- Aggregate related small improvements into single descriptive lines
- Keep entries concise: one line per feature/change, avoid technical jargon
- Focus on user benefits: what can they do now? what's better for them?
- This file must remain machine-parsable (Keep a Changelog format)
- The use of em dashes as punctuation is absolutely forbidden
-->

## [1.1.0-alpha] - Unreleased

### Added

- New default voice separation model: MelBand RoFormer. ~40% better quality than Demucs, requires 4GB VRAM
- Support for styled subtitle files (.ass/.ssa) commonly found in anime releases
- Support for subtitles embedded in video files (.mkv/.mp4), no manual extraction required

### Changed

- Voice separation now auto-detects your GPU: simplified dropdown with model names, NVIDIA checkbox which auto-enables when sufficient VRAM is available
- Systems with less than 4GB VRAM now default Demucs
- Model weights downloads now show detailed progress (e.g., "9.06M / 913M") and failed downloads are cleaned up automatically
- Clearer setup guidance for Dubtitles feature on fresh installs
- Process button stays visually active instead of being greyed out when blocked by configuration errors, ensuring the user will see the error tooltip

### Fixed

- Visual glitches in welcome popup animations when running inside Anki on Linux
- Progress bar now shows accurate completion for closed caption subtitle files
- Progress bar no longer accumulates counts when cancelling and restarting processing

## [1.0.1-alpha] - 2025-12-22

### Changed

- Local voice enhancing now handles any file length on Nvidia GPUs with as little as 2GB VRAM through automatic segmentation of audio according to your GPU's VRAM

## [1.0.0-alpha] - 2025-12-19

### Added

- Local voice enhancing option using Demucs: runs entirely on your computer for free, significantly faster with GPU acceleration
- Custom STT and LLM endpoint options for power users for local inference
- Fast & accurate local Thai romanization using Paiboon style: no scraping of thai2english needed anymore!
- Lite mode option replacing blurs to address flickering issue of when running inside Anki on Windows
- Warning dialog when hardware acceleration is unavailable, with guidance on how to fix it

### Changed

- Faster and more reliable setup: all docker-based processing tools now download as pre-built packages with download progress bar to keep users aware of the installation process
- More robust Anki integration with better detection of hardware and configuration issues
- Progress bars now highlight the most relevant task based on your selected features
- Subtitle detection now recognizes Chinese script variants (Simplified/Traditional) and other regional tags

### Fixed

- Progress bars for transliteration Russian & Chinese now show correct completion percentage and better organized romanization scheme
- On webkit: Drag and drop now works anywhere in the window, not just the top-left area
- Romanization of chinese subtitles now longers throws an error on Windows
- Cancel button works reliably when clicked right after starting
- Degraded audio quality of OPUS audio files introduced in 0.9.3-alpha
