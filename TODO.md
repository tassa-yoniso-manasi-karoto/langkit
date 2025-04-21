
SVG animations:
1. ~~restore static cancel appearance~~
2. ~~add micro-palette for each state and tweak gradients to use it + test visuals against original implementation~~
3. ~~add ***fancyMode*** and regular mode for limited computing + ***isWindowMinimized distinction*** that reverts to static gradients~~
4. rework state transitions

- remove useless test files

- progress bar
  - ETA calculation itself: use processedCount for ETA calculation
  - 🤯 make consistent progress bars between GUI and CLI
  
- go-rod
  - single Browser Access URL declaration
  - make currenlty running browser download known in the GUI
  
- wasm no memory estimates
 
- appStartCount→ countAppStart; save countAppStart in config and add countProcessStart

- GUI options:
  - condensed audio toggle
  - dubs allow user to request cached sep voice file deletion → in Settings
  
- coffee icon

- support Condensed Audio summaries

- manual GUI tests
- manual CLI tests
- try offline to see if icon / fonts are missing

- try a clean install


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

