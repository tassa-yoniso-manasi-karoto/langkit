- cleanup abandoned Wails methods

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
