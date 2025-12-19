- demucs local not loud enough

18:12:47 WRN Failed to stop Demucs container error="Get \"http://%2Fvar%2Frun%2Fdocker.sock/v1.51/containers/json?all=1&filters=%7B%22label%22%3A%7B%22com.docker.compose.config-hash%22%3Atrue%2C%22com.docker.compose.oneoff%3DFalse%22%3Atrue%2C%22com.docker.compose.project%3Dlangkit-demucs-gpu%22%3Atrue%7D%7D\": context canceled"


- Wave svg reset too fast make longer loop
- Fix useless log spam of LLM providers

- without leakless: https://github.com/go-rod/rod/issues/739#issuecomment-1272420000

- “Found ffmpeg for CLI task” in GUI mode, but it ‘s not so much a problem by itself i guess: can add a last resort check in AppData\Local\langkit\tools


- cleanup abandoned Wails methods
- support embedded subtitles
- ensure support .ass subtitles

- ichiran multithreaded?

- BROWSE FIXMEs / TODOs IN CODEBASE
- more manual GUI tests
- could do CLI test run of docker imgs in a Github workflow assuming all the CLI logic works (it doesn't)

.

- fix newlines in builtin documentation

.

- LINT
- REFACTOR CORE

<hr>

### future implementations

- 🚧🚧 add tlit in TSV/CSV
- 🚧🚧 draft word frequency list feature

.

- condensed audio
  - live edit summary prompt
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

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE NON GOLANG LIBS TO BE INSTALLED
- use Enhanced voice audiotrack as basis for audio clips
- lossless AVIF extraction from AV1 (HQ but worse than JPEG in size)
