### TODO
*in progress:*

- fork progressbar bc its time prediction use a rate based on few past seconds to make an ETA and it is garbage when tasks are CPU bound + massive task pool
- for bulk processing: leverage WithLevel() to implement --less-lethal
- (MUST TEST:) insanely-fast-whisper

*later:*

- add subtitle transliteration? remote API is difficult but so is shipping python with NLP libs. 🤔
	Tokenization needed: thai2english.com/pythaiNLP/deepcut (tha), ginza (jpn), HanLP (zh) (also no space: Lao, Burmese, Khmer, Tibetan.)
	Transliteration needed too: Korean, Arabic, Russian, indic languages (Hindi/Bengali at least), Cantonese
- Make autosub local-independent: en match if en-US, no match if en-US and en-IN. Add a --strict
- integrate with viper and yaml config file:
    - whisper initial_prompt
    - tokens
    - gain & limiter parameters for merging
- more debug info (FFmpeg version, mediainfo, platform...)
- with [libvips binding](https://github.com/h2non/bimg) fuzz trim to remove black padding if ratio is different

*might:*

- speechmatics (NO GO LIB) https://docs.speechmatics.com/introduction/batch-guide	 https://docs.speechmatics.com/jobsapi#tag/RetrieveTranscriptResponse
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR when standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

