### TODO

   - Add tests

   - crash reports:
     - ADD DOCKER LOGS
     - prefer zip over zstd because github issues don't support uplodad zstd
     - add snapshots of tsk throughout code
     - ls inside directory of media
     - bind it to CLI runs too
   
translit.go

   - USE SELECTIVETRANSLIT DIRECTLY FOR KANA TRANSLIT
   - set Translitkit logwriter to langkit's
   - fix IsToken situation
   - implement ctx support
   
   - more logging on langs detected
   - whisper initial_prompt
   - user-defined API retry max
   - gain & limiter parameters for merging
   - add progress bar
   
   
   - Add documentation
   - fix newlines in builtin documentation

   - use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)

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


- Make autosub local-independent: en match if en-US, no match if en-US and en-IN. Add a --stric
- with [libvips binding](https://github.com/h2non/bimg) fuzz trim to remove black padding if ratio is different

*might:*

- use Enhanced voice audiotrack as basis for audio clips
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

