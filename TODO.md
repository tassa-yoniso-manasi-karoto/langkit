### TODO
*in progress:*

- loop whisper with n retry
- make dubtitle file from whisper transcription
- resuming capabilites for whisper
- (MUST TEST:) insanely-fast-whisper
- verbose, padded mode for when iterating mp4 in a folder
- link static ffmpeg

*later:*

- integrate with viper and yaml config file:
    - whisper initial_prompt
    - tokens
    - gain & limiter parameters for merging


*might:*

- with [libvips binding](https://github.com/h2non/bimg) fuzz trim to remove black padding if ratio is different
- use Enhanced voice audiotrack as basis for audio clips
- more debug info (FFmpeg version, mediainfo, platform...)
- use lower bitrate opus with DRED & LBRR when standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

