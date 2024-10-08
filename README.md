### Status: prerelease

Fork of Bunkai, which reimplemented the functionality first pioneered by **cb4960** with [subs2srs](https://subs2srs.sourceforge.net/).

### Requirements
This fork require FFmpeg **v6 or higher (dev builds being prefered)**, Mediainfo, a [Replicate](https://replicate.com/home) API token.

The [FFmpeg dev team recommends](https://ffmpeg.org/download.html#releases) end-users to only use the [latest builds from the development branch (master builds)](https://github.com/BtbN/FFmpeg-Builds/releases). 

At the moment tokens should be passed through these env variables: REPLICATE_API_TOKEN, ELEVENLABS_API_TOKEN.

# Extra features of this fork

### Default encoding to OPUS / AVIF
Use modern codecs to save storage. The image/audio codecs which langkit uses are state-of-the-art and are currently in active development.

The static FFmpeg builds guarantee that you have up-to-date codecs. **If you don't use a well-maintained bleeding edge distro or brew, use the dev builds.**

### Automatic Speech Recognition / Speech-to-Text support
[Translations of recorded dubbings and of subtitles differ](https://www.quora.com/Why-do-subtitles-on-a-lot-of-dubbed-shows-not-match-up-with-the-dub-itself). Therefore dubs can't be used with the original subs2srs.<br>
With the flag `--stt` you can use [Whisper](https://github.com/openai/whisper) (v3-large) on the audio clips corresponding to timecodes of the subtitles to get the transcript of the audio and then, have it replace the translation of the subtitles.

### Condensed Audio
langkit will automatically make an audio file containing all the audio snippets of dialog in the audiotrack. <br>
This is meant to be used for passive listening. <br>
More explanations and context here: https://www.youtube.com/watch?v=QOLTeO-uCYU

### Enhanced voice audiotrack
Make a new audiotrack with voices louder by merging the original audiotrack with an audiotrack contatining the voices only.<br>
This is very useful for languages that are phonetically dense, such as tonal languages, or for languages that sound very different from your native language.<br>
<br>
The separated voices are obtained using one of these:

| Name <br>(to be passed with -s) | Quality of separated vocals | Price                          | Type        | Note                                                           |
|---------------------------------|-----------------------------|--------------------------------|-------------|----------------------------------------------------------------|
| demucs                          | good                        | very cheap<br>0.063$/run          | MIT license | **The one I'd recommend**                                      |
| demucs_ft                       | good                        | cheap<br>0.252$/run               | MIT license | Fine-tuned version: "take 4 times more time but might be a bit better". I couldn't hear any difference with the original in my test. |
| spleeter                        | rather poor                 | very, very cheap<br>0.00027$/run  | MIT license |                                                                |
| elevenlabs                      | good                        | very, very expensive<br>1$/minute | proprietary | Not fully supported due to limitations of their API (mp3 only) which desync the processed audio with the original. <br> **Requires an Elevenlabs API token.** <br> Does more processing than the others: noises are entirely eliminated, but it distort the soundstage to put the voice in the center. It might feel a bit uncanny in an enhanced track. |


## ...But why?
There are plenty of good options already: [Language Reactor](https://www.languagereactor.com/) (previously Language Learning With Netflix), [asbplayer](https://github.com/killergerbah/asbplayer), [mpvacious](https://github.com/Ajatt-Tools/mpvacious), [voracious](https://github.com/rsimmons/voracious), subs2srs, online sentence banks, high quality premade Anki decks...

Here is a list: https://github.com/nakopylov/awesome-immersion

They are really awesome but all of them are media-centric: they are implemented around watching shows and you make the most of them by watching a whole lot of shows.

I am more interested in an approach centered around words:
- **word frequency**: I learn either words I handpicked myself or words taken from the a list of most frequent words found in the content I am interested in and then sort them in groups of priority. I believe this is the most time efficient approach if you aren't willing to spend most of your time and energy on language learning.
- **word-centric notes referencing all common meanings**: one note to rule them all. I cross-source dictionaries and LLMs to the map the meanings, connotations and register of a word. Then I use another tool to search my database of subs to illustrate & disambiguate with real-world examples the meanings I have found.
- **word-note reuse for language laddering**: Another awesome advantage of this approach it that you can use this very note as basis for making cards for a new target language further down the line, while keeping all your previous note fields at hand for making the cards template for your new target language. The initial language acts just like Note ID for a meaning mapped across multiple languages. The majority of the basic vocabulary can be translated across languages directly with no real loss of meaning (and you can go on to disambiguate it further, using the method above for example). The effort that you spend on your first target language will thus pay off on subsequent languages.

**This is not meant to replace input or engaging with your target language but, on the SRS side of thing, I believe this is the most efficient time/effort investment to become familiar and knowledgeable enough about a word to use it in its correct, idiomatic meaning with confidence.**

There are several additional tools I made to accomplish this but they are hardcoded messes so don't expect me to publish them, langkit is enough work for me by itself! :)

### License
All new contributions from commit d540bd4 onward are licensed under GPL-3.0.

See original README of bunkai below for the basic features:
<hr>

Dissects subtitles and corresponding media files into flash cardsfor [sentence mining][1] with an [SRS][2] system like [Anki][3]. It is inspired
by the linked article on sentence mining and [existing tools][4], which you
might want to check out as well.

[1]: https://web.archive.org/web/20201220134528/https://massimmersionapproach.com/table-of-contents/stage-1/jp-quickstart-guide/
[2]: https://en.wikipedia.org/wiki/Spaced_repetition
[3]: https://ankiweb.net/
[4]: #known-alternatives

## Features
- **One or two subtitle files**: Two subtitle files can be used together to
  provide foreign and native language expressions on the same card.
- **Multiple subtitle formats**: Any format which is supported by [go-astisub][5]
  is also supported by this application, although some formats may work slightly
  better than others. If in doubt, try to use `.srt` subtitles.

[5]: https://github.com/asticode/go-astisub

## Installation
There is no proper release process at this time, nor a guarantee of stability
of any sort, as I'm the only user of the software that I am aware of. For now,
you must install the application from source.

Requirements:
- `go` command in `PATH` (only to build and install the application)
- `ffmpeg` command in `PATH` (used at runtime)

```bash
go get github.com/tassa-yoniso-manasi-karoto/langkit
```

## Usage
langkit is mainly used to generate flash cards from one or two subtitle files
and a corresponding media file.

For example:

```bash
langkit subs2cards media-content.mp4 foreign.srt native.srt
```

The above command generates the tab-separated file `foreign.tsv` and a
corresponding directory `foreign.media/` containing the associated images and
audio files. To do sentence mining, import the file `foreign.tsv` into a new
deck and then, at least in the case of Anki, copy the media files manually into
Anki's [collection.media directory](https://apps.ankiweb.net/docs/manual.html#file-locations).

Before you can import the deck with Anki though, you must add a new
[Note Type](https://docs.ankiweb.net/#/editing?id=adding-a-note-type)
which includes some or all of the fields below on the front and/or back of
each card. The columns in the generated `.tsv` file are as follows:

| # | Name | Description |
| - | ---- | ----------- |
| 1 | Sound | Extracted audio as a `[sound]` tag for Anki |
| 2 | Time | Subtitle start time code as a string |
| 3 | Source | Base name of the subtitle source file |
| 4 | Image | Selected image frame as an `<img>` tag |
| 5 | ForeignCurr | Current text in foreign subtitles file |
| 6 | NativeCurr | Current text in native subtitles file |
| 7 | ForeignPrev | Previous text in foreign subtitles file |
| 8 | NativePrev | Previous text in native subtitles file |
| 9 | ForeignNext | Next text in foreign subtitles file |
| 10 | NativeNext | Next text in native subtitles file |

When you review the created deck for the first time, you should go quickly
through the entire deck at once. During this first pass, your goal should be
to identify those cards which you can understand almost perfectly, if not for
the odd piece of unknown vocabulary or grammar; all other cards which are
either too hard or too easy should be deleted in this pass. Any cards which
remain in the imported deck after mining should be refined and moved into your
regular deck for studying the language on a daily basis.

For other uses, run `langkit --help` to view the built-in documentation.

## Subtitle editors
The state of affairs when it comes to open-source subtitle editors is a sad
one, but here's a list of editors which may or may not work passably. If you
know a good one, please let me know!

| Name | Platforms | Description |
| ---- | --------- | ----------- |
| [Aegisub](http://www.aegisub.org/) | macOS & others | Seems to have been a popular choice, but is no longer actively maintained. |
| [Jubler](https://www.jubler.org/) | macOS & others | Works reasonably well, but fixing timing issues is still somewhat cumbersome. |

## Known alternatives
There are at least three alternatives to this application that I know of, by
now. Oddly enough, I found substudy just after the prototype and movies2anki
when I published this repository. Something is off with my search skills! :)

- [movies2anki](https://github.com/kelciour/movies2anki]): Fully-integrated add-on for Anki which has some advanced features and supports all platforms
- [substudy](https://github.com/emk/subtitles-rs/tree/master/substudy): CLI alternative to subs2srs with the ability to export into other formats as well, not just SRS decks
- [subs2srs](http://subs2srs.sourceforge.net/): GUI software for Windows with many features, and inspiration for substudy and Bunkai
