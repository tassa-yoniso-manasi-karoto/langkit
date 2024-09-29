### Status: prerelease

Fork of Bunkai, which reimplemented the functionality first pioneered by **cb4960** with [subs2srs](https://subs2srs.sourceforge.net/).

### Requirements
This fork require ffmpeg **version 6 or higher**, Mediainfo, a [Replicate](https://replicate.com/home) API token.

At the moment tokens should be passed through these env variables: REPLICATE_API_TOKEN, ELEVENLABS_API_TOKEN.

### TODO
- link static ffmpeg for windows
- loop whisper with n retry
- make `--stt` into string and add incredibly-fast-whisper as alternative
- auto subs selection based on ISO lang and display warning if `--stt` is used when dubtitles or CC for that lang are detected
- integrate with viper and yaml config file:
    - whisper initial_prompt
    - tokens
    - gain & limiter parameters for merging


*might:*
- use Enhanced voice audiotrack as basis for audio clips
- use lower bitrate opus with DRED & LBRR when standardized [1](https://opus-codec.org/),[2](https://datatracker.ietf.org/doc/draft-ietf-mlcodec-opus-extension/)

## Extra features of this fork

### Default encoding to OPUS / AVIF
Use modern codecs to save storage

### Automatic Speech Recognition / Speech-to-Text support
[Translations of recorded dubbings and of subtitles differ](https://www.quora.com/Why-do-subtitles-on-a-lot-of-dubbed-shows-not-match-up-with-the-dub-itself). Therefore dubs can't be used with the original subs2srs.<br>
With the flag `--stt` you can use [Whisper](https://github.com/openai/whisper) (v3-large) on the audio clips corresponding to timecodes of the subtitles to get the transcript of the audio and then, have it replace the translation of the subtitles.

### Condensed Audio
subs2cards will automatically make an audio file containing all the audio snippets of dialog in the audiotrack. <br>
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

### License
All new contributions from commit d540bd4 onwards are licensed under GPL-3.0.

See original README below:
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
- **Media files are optional**: Requires only a single foreign subtitles file to
  generate text-only flash cards. Associated media content is optional, but
  highly recommended.
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
go get github.com/tassa-yoniso-manasi-karoto/subs2cards
```

## Usage
subs2cards is mainly used to generate flash cards from one or two subtitle files
and a corresponding media file.

For example:

```bash
subs2cards extract cards -m media-content.mp4 foreign.srt native.srt
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

For other uses, run `subs2cards --help` to view the built-in documentation.

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

## Change log
See the file [CHANGELOG.md](CHANGELOG.md).
