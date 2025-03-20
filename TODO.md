

- sub roma ichiran not written
- migrate go-ichiran/aksharamukha to use ctx

- GUI options:
  - condensed audio toggle
  - dubs allow user to request cached sep voice file deletion

- 🤯 consistent progress bars between GUI and CLI
  - processedCount for ETA calculation
  - ETA algo bc progressbar pkg uses a rate-based on few past seconds to make an ETA and it is garbage when tasks are CPU bound + massive task pool

- "Recreate Docker containers" should not show everywhere →→→ SELECTOR #L472
- truncated feature card (left & right) when not maximized

- crash reports add snapshots of tsk throughout code
- manual GUI tests
- manual CLI tests
- try offline to see if icon / fonts are missing


- translitkit close
- add progressCallback to all providers in translitkit
- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase


*future implementations*

- implement an explicit maxAbortTasks

- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

- word freq lists

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

