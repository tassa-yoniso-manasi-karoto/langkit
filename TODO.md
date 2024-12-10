### TODO
*in progress:*

- WARN USERS IN DOCS ABOUT REPLICATE ERRORS FOR DEMUCS / SPLEETER
- add subtitle transliteration? remote API is difficult but so is shipping python with NLP libs. 🤔
	Tokenization needed: deepcut (tha), ginza (jpn), HanLP (zh) (also no space: Lao, Burmese, Khmer, Tibetan.)
- fork progressbar bc its time prediction use a rate based on few past seconds to make an ETA and it is garbage when tasks are CPU bound + massive task pool
- MUST CHECK integrity of multiplexed tsv
- for bulk processing: leverage WithLevel() to implement --less-lethal
- create merged track in dir of video instead of the video.media directory that s2s creates → automatic selection with mpv
- (MUST TEST:) insanely-fast-whisper

*later:*

- integrate with viper and yaml config file:
    - whisper initial_prompt
    - tokens
    - gain & limiter parameters for merging
- tag merged audiotrack

*might:*

- speechmatics (NO GO LIB) https://docs.speechmatics.com/introduction/batch-guide	 https://docs.speechmatics.com/jobsapi#tag/RetrieveTranscriptResponse
- with [libvips binding](https://github.com/h2non/bimg) fuzz trim to remove black padding if ratio is different
- use Enhanced voice audiotrack as basis for audio clips
- more debug info (FFmpeg version, mediainfo, platform...)
- use lower bitrate opus with DRED & LBRR when standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

