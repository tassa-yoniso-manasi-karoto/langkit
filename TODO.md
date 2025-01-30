### TODO

UI:
   - check that Docker Desktop/Engine is available, if jp/indic translit
   - Add hovertips

TRANSLITKIT:
   - implement ctx support
   - Transliteration: LIST STYLES AVAILABLE



   - Add tests
   - Add tooltips (aborted tasks...)
   - add progress bar
   
   - Add documentation
   - fix newlines in builtin documentation



*in progress:*
- add subtitle transliteration? remote API is difficult but so is shipping python with NLP libs. 🤔
https://awesome-go.com/tokenizers/
https://go.libhunt.com/
	Thai:
		PythaiNLP + my own lib?
	Japanese:	https://github.com/taishi-i/awesome-japanese-nlp-resources/
		ikawaha / kagome
		❌ shogo82148 / go-mecab : above should be enough
		ginza (py)
		Kanji translit: https://github.com/ysugimoto/go-kakasi
		Kana-romaji transliterator: robpike / nihongo  OR  gojp / kana 
	Chinese: 
		Tokenizer https://github.com/yanyiwu/gojieba
		Transliterator https://github.com/mozillazg/go-pinyin or https://github.com/mozillazg/go-unidecode (same author)
	
	Transliteration needed too: Arabic, Cantonese
- fork progressbar bc its time prediction use a rate based on few past seconds to make an ETA and it is garbage when tasks are CPU bound + massive task pool
- for bulk processing: leverage WithLevel() to implement --less-lethal
- (MUST TEST:) insanely-fast-whisper

*later:*


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

