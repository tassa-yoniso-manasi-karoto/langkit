🔳 `Subtitle lines processed (all files)...- [ETA: 3m-15m (92%)]` -> in bulk mode, this bar is being recreated at each file
🔳 processing JPN should not be bigger size than processing voice sep

- ENHANCED as source for:
  - condensed audio
  - subs2cards audio clips

- Data aware logic:
  - collapse "Corrupt Audio Tracks" WITH "Audio Decode Failures"
  - 🚧 groom = put irrelevant subs to target dir & demux and mov irrelevant audio tracks

- "Do-no-interrupt" mode: <<<--- IMPLEMENT AS PART OF THE PRE-FLIGHT  CHECK: the user knows the problem immediately at processing starts instead auto-tolerating mid-run
   - this error should abort_all: "Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe ('). Apostrophe in the names of the files themselves are supported using a workaround."
   - should NOT abort_all, give it leaway: "selecting audiotrack: No audiotrack tagged with the requested target language exists. If it isn't a misinput please use the audiotrack override to set a track number manually."

- groups of "autosubs failed: no subtitle matching target language Japanese was found" could be mutualized in a user-friendly "meta-error" displayed in progress manager: "Folder X does not appear to contain subtitles in language Y."

- 🐢🐢🐢 Wave svg reset too fast make longer loop

- 🚧 add tlit in TSV/CSV

- ASR/STT Benchmarker

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

.

- implement an explicit maxAbortTasks
- add progressCallback to all providers in translitkit including go-natives

.

- hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
- ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes

<hr>

*might:*

- with [libvips binding](https://github.com/h2non/bimg) or imagor: fuzz trim to remove black padding if ratio is different => REQUIRE NON GOLANG LIBS TO BE INSTALLED
- lossless AVIF extraction from AV1 (HQ but worse than JPEG in size)
