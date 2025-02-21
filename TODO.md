
- ctx canceled in Whisper???


transliteration:
   - selective translit rework
   - selective translit as a separate feature only for jpn



- UI: 
  - add progress bar (



translitkit
   - gojieba + go-pinyin





- implement CloseAll()
- crash reports add snapshots of tsk throughout code


- Add tests
   - user-defined API retry max
   
   - fix directory tree: branches off center
   - fix newlines in builtin documentation

→→→ Wait for Claude 4 release soon to break large functions (Execute and ProcessItem) and write some tests.

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
- (MUST TEST:) insanely-fast-whisper

*later:*


- with [libvips binding](https://github.com/h2non/bimg) fuzz trim to remove black padding if ratio is different

*might:*

- Make autosub local-independent: en match if en-US, no match if en-US and en-IN. Add a --stric
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR that were just standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)
- lossless AVIF extraction from AV1 (HQ but worst than JPEG in size)

