### TODO

- Add logging to the LogViewer component
- Add error Handling with more or less critical ERR that may cause a a particular task to be but not all of them, or cancel the run and display a warning

- then:
   - Add File Drop Zone Handling: file validation and preprocessing
   - Add Settings Persistence with viper (or koanf?)
   - Add tooltips and help documentation
   - add progress bar
   - Implement cancel capability
 
  <hr>

  
<table><thead>
  <tr>
    <th></th>
    <th>subs2cards</th>
    <th>subs2dubs</th>
    <th>enhance</th>
    <th>translit</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make tokenized subtitle</td>
    <td>✅</td>
    <td>🚫</td>
    <td>❌</td>
    <td>✅</td>
  </tr>
  <tr>
    <td>Make translit subtitle</td>
    <td>✅</td>
    <td>🚫<br></td>
    <td>❌<br></td>
    <td>✅<br></td>
  </tr>
  <tr>
    <td>Make enhanced track</td>
    <td>✅</td>
    <td>✅<br></td>
    <td>✅</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make a merged video</td>
    <td>✅</td>
    <td>✅</td>
    <td>✅</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized dubtitle</td>
    <td>✅</td>
    <td>✅</td>
    <td>❌</td>
    <td>🚫<br></td>
  </tr>
  <tr>
    <td>Make translit dubtitle</td>
    <td>✅</td>
    <td>✅</td>
    <td>❌</td>
    <td>🚫<br></td>
  </tr>
  <tr>
    <td>Make dubtitle</td>
    <td>✅</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌<br></td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
</tbody></table>


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

