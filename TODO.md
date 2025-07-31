- fix chrome/qtWebEngine specific bug: isProcessing not set properly

.
- welcomepop displays forever, ignoring countAppStart
- autoscroll won't reenable when scrolling down.

.

- ⭐ ANKI ADDON
- ⭐ PYTHAILNLP

.
- BROWSE FIXMEs / TODOs IN CODEBASE
- manual GUI tests
  - check settings panel from a non dev perspective

.

- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase

.

- cleanup abandoned wails methods
- LINT
- REFACTOR CORE

<hr>

### future implementations

- 🚧🚧 add tlit in TSV/CSV
- 🚧🚧 draft word frequency list feature

.
backoff requests using https://github.com/cenkalti/backoff

.

- condensed audio
  - multiple previous subtitle for summarization fully contextualized (Providing x previous subtitles or their summaries of previous episodes as context)
  - ENHANCE condensed audio

.

- implement an explicit maxAbortTasks
- add progressCallback to all providers in translitkit including go-natives

.

- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

<hr>

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that are not yet standardized [ietf draft](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/),expires October 31 2025, check again later
- lossless AVIF extraction from AV1 (HQ but worse than JPEG in size)
