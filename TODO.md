SVG animations: rework state transitions

- 🤯 make consistent progress bars between GUI and CLI

- go-rod
  - single Browser Access URL declaration
  - make currenlty running browser download known in the GUI

- condensed audio: CHECK IT

- support Condensed Audio summaries:
  - perf issue with queries of GetAvailableModels()
  
  - 🔳 manually fix AI slop in translateReq2Tsk; double check it against DEV.md
  - hardcode padded timming to 250ms (IIRC it found it ideal to not get truncated, or overlapping sentences)
  - "Custom Summary Prompt" → should have bigger field like Initial Prompt
.

- add tlit in TSV/CSV

.

- remove assemblyAI
.

- guarantee all settings in Settings panel work
.

- can you make sure env set API keys are heeded in GUI mode?

.
- ProcessErrorTooltip should have a fade in / out

.
- welcome.svelte component w/ checks for binary needed

- add strategic frontend logging for prod
.

- std language string to lowercase and support language names too

.
- remove useless test files
.

- manual GUI tests
- manual CLI tests
- try offline to see if icon / fonts are missing

- try a clean install
.

- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase
.





<hr>


*future implementations*

- condensed audio
  - multiple previous subtitle for summarization fully contextualized (Providing x previous subtitles or their summaries of previous episodes as context)
  - ENHANCE condensed audio

,

- implement an explicit maxAbortTasks
- add progressCallback to all providers in translitkit including go-natives

- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

- word freq lists

<hr>

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

