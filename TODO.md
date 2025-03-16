
- NORMALIZE / BALANCE UI COLORS (process manager, error tooltip too)

- progressbar not destroyed in time on resume
- no update when minimized
- sub roma ichiran not written

- implement an explicit maxAbortTasks
- hide logviewer and alert on error
- immutable context problem

- GUI options:
  - condensed audio toggle
  - dubs allow user to request cached sep voice file deletion

- frontend logs in log viewer
- crash reports add snapshots of tsk throughout code
- manual GUI tests
- manual CLI tests

- translitkit close
- progressCallback for all providers
- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase

*later:*

- fork progressbar bc its time prediction use a rate based on few past seconds to make an ETA and it is garbage when tasks are CPU bound + massive task pool
- make sure API retries are subject to ctx cancelation
- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

- word freq lists

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

