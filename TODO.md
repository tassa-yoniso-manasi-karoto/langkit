- PROGRESSBARS:
  - same class = single instance
  - 🚨🚨🚨 processing JPN should not be bigger size than processing voice sep
  
- tell user what to do if backend dies

- 🚨🚨🚨 should NOT abort_all, give it leaway: "selecting audiotrack: No audiotrack tagged with the requested target language exists. If it isn't a misinput please use the audiotrack override to set a track number manually."

- 🚨🚨🚨 romanized.srt get their ' trimmed

- 🐢🐢🐢 Wave svg reset too fast make longer loop

 🚧 groom = put irrelevant subs to target dir & demux and mov irrelevant audio tracks

- 🚧 add tlit in TSV/CSV
- 🚧 use MKVtoolnix to merge outputs WHILE PRESERVING TAGS & METADATA
  - A-V time shift
  - sub time shift

- Fix useless log spam of LLM providers

- without leakless: https://github.com/go-rod/rod/issues/739#issuecomment-1272420000


- ichiran multithreaded?

- BROWSE FIXMEs / TODOs IN CODEBASE
- more manual GUI tests
- could do CLI test run of docker imgs in a Github workflow assuming all the CLI logic works (it doesn't)

.

- fix newlines in builtin documentation

.

- LINT

<hr>

### future implementations

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
