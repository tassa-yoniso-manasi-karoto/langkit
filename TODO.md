	- "no language passed" log

transliteration:
   - selective translit as a separate feature only for jpn
   - update UI with "Selective transliteration" for jpn


- UI: 
  - hard limiter for workers num when making dubtitles from remote API (otherwise too many requests may induce delays and trigger timeouts)
  - gui: single video picker not connected to Autosub logic??
  - add progress bar
  - remove round around button to close settings
  - sliding appearance of features' cards
  - spinner of "Processing..." causes weird border effect
  - homogenize colors of log level with green check mark etc, paler blue debug level

- dubs allow user to request cached sep voice file deletion

translitkit
   - gojieba + go-pinyin



   - ichiran: add database corrupted warning


- implement CloseAll()
- crash reports add snapshots of tsk throughout code


   - ideally, scraper-providers should have exponential backoff both in timing and in their chunks' sizes
- Add tests
   
   - fix directory tree: branches off center
   - fix newlines in builtin documentation
   - refactor r8RunWithAudioFile with retrypolicy

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

