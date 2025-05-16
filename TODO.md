SVG animations: rework state transitions

- 🤯 make consistent progress bars between GUI and CLI

- go-rod
  - single Browser Access URL declaration
  - make currenlty running browser download known in the GUI

- condensed audio: CHECK IT

- support Condensed Audio summaries:
  - Implement API Key Handling: Integrate pkg/llms/registry.go with internal/config/settings.go to securely load and use API keys for OpenAI, LangChain (if it wraps specific key-based models), and OpenRouter.
  - Implement One LLM Provider Fully: Start with pkg/llms/openai.go. Implement the Complete method to make actual API calls to OpenAI using their Go SDK. This will serve as a template for other providers.
  - Refine summary.PrepareSubtitlesForSummary: Test with various subtitle formats and content to ensure the text fed to the LLM is clean and effective. Consider maximum input length for LLMs.
  - Integrate media.AddMetadataToAudio: Ensure this works reliably across different audio players with the "lyrics" tag. (MP3/AAC differences)
  - Develop GUI Components: Create UI elements for the user to enable summarization and choose provider/model.
  
- add tlit in TSV/CSV

- guarantee all settings in Settings panel work




- add strategic frontend logging for prod

- remove useless test files

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

