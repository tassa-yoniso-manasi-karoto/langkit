SVG animations: rework state transitions

- 🤯 make consistent progress bars between GUI and CLI

.
- cleanup logger.ts junk
14:01:42 TRC FRONT: GroupOption mounted: subtitle.style=paiboon component=groupOption fe_appVersion=dev fe_sessionId=mbag5aul9u4q27l fe_timestamp=1748587998765 fe_userAgent="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15 wails.io/605.1.15" fe_viewport=1030x724 module=app sessionId=mbag5aul9u4q27l

.

- condensed audio:
  - watermark model name in lyrics
  - write concat WAV to tmp for easy retries
  - backoff requests

.
- add tlit in TSV/CSV

.
- guarantee all settings in Settings panel work
  - implement intermediary file mode

.
- welcome.svelte component w/ checks for binary needed:
  - less transparency
  - use ExternalLink
  - write "if ... then features X and Y are unavailable msg"

.
- mergeInsertLogs calls (2nd check)

.
- remove useless test files
.

- finish theme 

.
- final check to add strategic frontend logging for prod (2nd check)
  - change levels of logs "Created new provider pool" in translit manager

.
- revoke API keys and make sure they are under no circumstances leaking in logs

.

- have claude scan for perf improvemnts / optimizations
- manual GUI tests
  - double check translateReq2Tsk against DEV.md
  - watch for combinations that should spawn an error and disallow processing:
    - no docker but japanese transliteration selected
    - Speech-to-Text and voice enhancing will not be available offline
  - test use case BrowserAccessURL not set 2 time in a row (test logger.go with already downloaded binary)
- manual CLI tests
- try offline to see if icon / fonts are missing

- try a clean install
.

- fix newlines in builtin documentation
- Browse / check FIXMEs in codebase
.

- chk memory management of WASM (remove 50Mib preallocated)




```
Looking at the logs, this error is not critical but shows a race condition during startup:

  10:41:34 DBG LLM state change emitted global_state=ready
  10:41:34 DBG Received LLM registry state change component=summary global_state=ready
  10:41:34 INF LLM registry is ready, updating summary providers component=summary
  10:41:34 WRN GetClient: Registry not ready. component=llm_registry global_state=updating
  10:41:34 ERR Failed to get LLM client from registry error="llm registry: system not ready" component=summary

  What's happening:
  1. LLM registry finishes initialization and becomes "ready"
  2. Summary service receives the "ready" notification and tries to update providers
  3. But simultaneously, the frontend saves settings on startup, triggering a registry update 		// FIXME → this is most likely counterAppStart
  4. By the time summary service calls GetClient(), registry is already back in "updating" state

  Should you be concerned?
  No, this is not critical:
  - The summary service will retry when the registry becomes ready again after the update
  - The system will work correctly once the second initialization completes
  - It's just inefficient during startup

  The issue:
  The frontend is triggering a settings save immediately on startup, which causes an unnecessary re-initialization of the already-ready LLM registry.

  This could be optimized by:
  1. Not triggering settings updates during initial startup if nothing actually changed
  2. Making the summary service more resilient to temporary registry state changes
  3. Debouncing settings updates to avoid rapid re-initialization

  For now, you can ignore this error - it's just startup inefficiency, not a functional problem.

```


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

