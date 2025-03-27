
- bug in TH2EN

- Adaptive Event Throttling System
  - virtual log view → must std log entry height
  - progressbar no updating under low load

- go-rod
  - atm valid browser URL required even for Subs2Cards alone
  - make browser url option to use go-rod's ability to download & manage the instance for noobs


- progress bar
  - subline total not reset on resume
  - 🤯 make consistent progress bars between GUI and CLI
  - correct sub line count, even accross resume
  - processedCount for ETA calculation
  - ETA calculation itself
 
- appStartCount→ countAppStart; save countAppStart in config and add countProcessStart
- use selective translit tokenized inside TRANSLIT.GO

- GUI options:
  - condensed audio toggle
  - dubs allow user to request cached sep voice file deletion
  
- coffee icon

- support Condensed Audio summaries

- manual GUI tests
- manual CLI tests
- try offline to see if icon / fonts are missing
- clean run in VirtualBox


- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase


*future implementations*

- implement an explicit maxAbortTasks
- add progressCallback to all providers in translitkit including go-natives

- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

- word freq lists

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

