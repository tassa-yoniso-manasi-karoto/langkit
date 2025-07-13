BUGS TO CONFIRM:
- translit: "Subtitle lines processed (all files)... 0/1" forever on a single media file run
- selective translit doesn't briefly show "sorry not available" on webview2

.

- mediainput unhandled err → better err
- broken feature message on webview2

- broken feature message on WEBKIT. /!\
- go-ichiran register user cancel as: 14:58:18 WRN Transliteration provider marked as unhealthy due to processing error error="error analyzing chunk 1: failed to read exec output: no valid JSON line found in output" component=provider_manager provider_key=jpn:Hepburn provider_name=jpn-ichiran
- crash report fallback API key sanitizer (to guarantee no leaks)
- truncate WS logs: FRONT: Message received [HUGE STRING]
- ask Claude thorough critic

.
- finish theme
  - rm unused selector
  - slightly bigger text on cards, perhaps more white also (better contrast)
  - scale up on hover for cards + ideally a black shadow like GH Ctrl+F
  - welcome.svelte component: increase borders
  - scale & rotate coffee some more

.
- LINT
- update dev.md with build instructions, designs explained in brief

.
- REFACTOR CORE

.
- PYTHAILNLP

.
- BROWSE FIXMEs / TODOs IN CODEBASE
- manual GUI tests
  - make consistent progress bars between GUI and CLI
  - check settings panel from a non dev perspective
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

- test more memory management of WASM (remove 50Mib preallocated)




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

