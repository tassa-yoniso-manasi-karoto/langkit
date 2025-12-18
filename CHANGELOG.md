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

## [1.0.0-alpha] - Unreleased

### Added

- Local voice enhancing option using Demucs: runs entirely on your computer at no cost, significantly faster with GPU acceleration
- Custom STT and LLM endpoint options for power users for local inference
- Fast & accurate local Thai romanization using Paiboon style: no scraping of thai2english needed anymore!
- Lite mode option replacing blurs to address flickering issue of when running inside Anki on Windows
- Warning dialog when hardware acceleration is unavailable, with guidance on how to fix it

### Changed

- Faster and more reliable setup: all docker-based processing tools now download as pre-built packages with download progress bar to keep users aware of the installation process
- More robust Anki integration with better detection of hardware and configuration issues
- Progress bars now highlight the most relevant task based on your selected features

### Fixed

- Progress bars for fast transliteration (Russian, Chinese) now show correct completion percentage
- Drag and drop now works anywhere in the window, not just the top-left area
- Subtitle detection now recognizes Chinese script variants (Simplified/Traditional) and other regional tags
- Romanization of chinese now longers throws an error on windows
- Specific language variants are now respected: requesting de-AT no longer matches de-DE subtitles
- Cancel button responds immediately even when clicked right after starting
