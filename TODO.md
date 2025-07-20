- mediainput unhandled err → better err: Recently @internal/gui/frontend/src/components/MediaInput.svelte  has been failing to display content of selected directory properly. Investigate.
- replace emoji by lottie https://googlefonts.github.io/noto-emoji-animation/ "Wave"

- "log viewer available" popping up after errors logged in logviewer (OUTSIDE OF PROCESSING)
  - non dismissable too

- single instance (due to webRPC/WS instead of webengine APIs)
- fix log levels spamming useless

.
- LINT
- rm unused selector

.
- cleanup abandonned wails methods & finish webrpc migration
- memoize validateTargetlang
- autoscroll won't reenable when scrolling down.
- PYTHAILNLP
- REFACTOR CORE

.
- BROWSE FIXMEs / TODOs IN CODEBASE
- manual GUI tests
  - check settings panel from a non dev perspective
  - watch for combinations that should spawn an error and disallow processing:
    - Speech-to-Text and voice enhancing will not be available offline
- try offline to see if icon / fonts are missing

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

