- fix log levels spamming useless
- cloudflare 2 bucket for nightlies
- welcomepop displays forever ignoring countAppStart
- build.yml add "Give executable permission to run it (chmod +x)"

.
- LINT

.
- cleanup abandoned wails methods & finish webrpc migration
- memoize validateTargetlang
- autoscroll won't reenable when scrolling down.

- ⭐ ANKI ADDON
- ⭐ PYTHAILNLP
- REFACTOR CORE

.
- BROWSE FIXMEs / TODOs IN CODEBASE
- manual GUI tests
  - check settings panel from a non dev perspective
- try offline to see if icon / fonts are missing

.

- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase

.
- test more memory management of WASM (remove 50Mib preallocated)




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

