<p align="center">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-blackBg.webp">
        <source media="(prefers-color-scheme: light)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
        <img width=375 src="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
    </picture>
</p>

### Status: prerelease

Fork of Bunkai, which reimplemented the functionality first pioneered by **cb4960** with [subs2srs](https://subs2srs.sourceforge.net/).

# tldr

```
𝗕𝗮𝘀𝗶𝗰 𝘀𝘂𝗯𝘀𝟮𝘀𝗿𝘀 𝗳𝘂𝗻𝗰𝘁𝗶𝗼𝗻𝗮𝗹𝗶𝘁𝘆
$ langkit subs2cards media.mp4 media.th.srt media.en.srt

𝗔𝘂𝘁𝗼𝗺𝗮𝘁𝗶𝗰 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘀𝗲𝗹𝗲𝗰𝘁𝗶𝗼𝗻 (𝘩𝘦𝘳𝘦: 𝘭𝘦𝘢𝘳𝘯 𝘣𝘳𝘢𝘻𝘪𝘭𝘪𝘢𝘯 𝘱𝘰𝘳𝘵𝘶𝘨𝘦𝘴𝘦 𝘧𝘳𝘰𝘮 𝘤𝘢𝘯𝘵𝘰𝘯𝘦𝘴𝘦 𝘰𝘳 𝘪𝘧 𝘶𝘯𝘢𝘷𝘢𝘪𝘭𝘢𝘣𝘭𝘦, 𝘵𝘳𝘢𝘥𝘪𝘵𝘪𝘰𝘯𝘢𝘭 𝘤𝘩𝘪𝘯𝘦𝘴𝘦)
$ langkit subs2cards media.mp4 -l "pt-BR,yue,zh-Hant"

𝗕𝘂𝗹𝗸 𝗽𝗿𝗼𝗰𝗲𝘀𝘀𝗶𝗻𝗴 (𝗿𝗲𝗰𝘂𝗿𝘀𝗶𝘃𝗲)
$ langkit subs2cards /path/to/media/dir/  -l "th,en"

𝗠𝗮𝗸𝗲 𝗮𝗻 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝘄𝗶𝘁𝗵 𝗲𝗻𝗵𝗮𝗻𝗰𝗲𝗱/𝗮𝗺𝗽𝗹𝗶𝗳𝗶𝗲𝗱 𝘃𝗼𝗶𝗰𝗲𝘀 𝗳𝗿𝗼𝗺 𝘁𝗵𝗲 𝟮𝗻𝗱 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝗼𝗳 𝘁𝗵𝗲 𝗺𝗲𝗱𝗶𝗮 (𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘯𝘦𝘦𝘥𝘦𝘥)
$ langkit enhance media.mp4 -a 2 --sep demucs

𝗠𝗮𝗸𝗲 𝗮 𝗱𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝗼𝗳 𝘁𝗵𝗲 𝗺𝗲𝗱𝗶𝗮 𝘂𝘀𝗶𝗻𝗴 𝗦𝗧𝗧 𝗼𝗻 𝘁𝗵𝗲 𝘁𝗶𝗺𝗲𝗰𝗼𝗱𝗲𝘀 𝗼𝗳 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗱 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝗳𝗶𝗹𝗲 (𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘯𝘦𝘦𝘥𝘦𝘥)
$ langkit subs2dubs --stt whisper media.mp4 (media.th.srt) -l "th"

𝗖𝗼𝗺𝗯𝗶𝗻𝗲 𝗮𝗹𝗹 𝗼𝗳 𝘁𝗵𝗲 𝗮𝗯𝗼𝘃𝗲 𝗶𝗻 𝗼𝗻𝗲 𝗰𝗼𝗺𝗺𝗮𝗻𝗱
$ langkit subs2cards /path/to/media/dir/  -l "th,en" --stt whisper --sep demucs
```

### Requirements
This fork require FFmpeg **v6 or higher (dev builds being preferred)**, Mediainfo, a [Replicate](https://replicate.com/home) API token.

The FFmpeg dev team recommends end-users to use only the latest [builds from the dev branch (master builds)](https://github.com/BtbN/FFmpeg-Builds/releases). The FFmpeg binary's location can be provided by a flag, in $PATH or in a "bin" directory placed in the folder where langkit is.

At the moment tokens should be passed through these env variables: REPLICATE_API_TOKEN, ASSEMBLYAI_API_KEY, ELEVENLABS_API_TOKEN.

# Extra features of this fork

### Default encoding to OPUS / AVIF
Use modern codecs to save storage. The image/audio codecs which langkit uses are state-of-the-art and are currently in active development.

The static FFmpeg builds guarantee that you have up-to-date codecs. **If you don't use a well-maintained bleeding edge distro or brew, use the dev builds.** You can check your distro [here](https://repology.org/project/ffmpeg/versions).

### Automatic Speech Recognition / Speech-to-Text support
Translations of dubbings and of subtitles differ.[¹](https://www.quora.com/Why-do-subtitles-on-a-lot-of-dubbed-shows-not-match-up-with-the-dub-itself) Therefore dubbings can't be used with subtitles in the old subs2srs unless said subs are closed captions or dubtitles.<br>
With the flag `--stt` you can use [Whisper](https://github.com/openai/whisper) (v3-large) on the audio clips corresponding to timecodes of the subtitles to get the transcript of the audio and then, have it replace the translation of the subtitles. AFAIK Language Reactor was the first to combine this with language learning from content however I found the accuracy of the STT they use to be unimpressive.

By default **a dubtitle file will also be created from these transcriptions.**



| Name (to be passed with --stt) | Word Error Rate average across all supported langs (june 2024) | Number of languages supported        | Price        | Type        | Note                                                                                                                                                                                                                                                                                                                   |
|--------------------------------|-----------------|-------------------------------------------------------------------------------------|--------------|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| whisper, wh                    | 10,3%           | [57](https://platform.openai.com/docs/guides/speech-to-text/supported-languages%5C) | $1.1/1000min | MIT         | **See [here](https://github.com/openai/whisper/discussions/1762) for a breakdown of WER per language.**  |
| insanely-fast-whisper, fast    | 16,2%           | 57                                                                                  | $0.0071/run  | MIT         |                                                                                                                                                                                                                                                                                                                        |
| universal-1, u1                | 8,7%            | [17](https://www.assemblyai.com/docs/getting-started/supported-languages)           | $6.2/1000min | proprietary | **Untested** (doesn't support my target lang)                                                                                                                                                                                                                                                                          |

See  [ArtificialAnalysis](https://artificialanalysis.ai/speech-to-text) and [Amgadoz @Reddit](https://www.reddit.com/r/LocalLLaMA/comments/1brqwun/i_compared_the_different_open_source_whisper/) for detailed comparisons.

Note: OpenAI just released a [turbo](https://github.com/openai/whisper/discussions/1762) model of large-v3 but they say it's on a par with large-v2 as far as accuracy is concerned so I won't bother to add it.
### Condensed Audio
langkit will automatically **make an audio file containing all the audio snippets of dialog** in the audiotrack. <br>
This is meant to be used for passive listening. <br>
More explanations and context here: [Optimizing Passive Immersion: Condensed Audio - YouTube](https://www.youtube.com/watch?v=QOLTeO-uCYU)

### Enhanced voice audiotrack
**Make a new audiotrack with voices louder**. This is very useful for languages that are phonetically dense, such as tonal languages, or for languages that sound very different from your native language.<br>
<br>
It works by merging the original audiotrack with an audiotrack containing the voices only.
<br>
The separated voices are obtained using one of these:

| Name (to be passed with --sep) | Quality of separated vocals | Price                               | Type        | Note                                                                                                                                 |
|--------------------------------|-----------------------------|-------------------------------------|-------------|--------------------------------------------------------------------------------------------------------------------------------------|
| demucs, de                     | good                        | very cheap 0.063$/run               | MIT license | **Recommended**                                                                                                            |
| demucs_ft, ft                  | good                        | cheap 0.252$/run                    | MIT license | Fine-tuned version: "take 4 times more time but might be a bit better". I couldn't hear any difference with the original in my test. | 
| spleeter, sp                   | rather poor                 | very, very cheap 0.00027$/run       | MIT license |                                                                                                                                      |
| elevenlabs, 11, el             | good                        | very, very expensive<br>1$/*MINUTE* | proprietary | Not fully supported due to limitations of their API (mp3 only) which desync the processed audio with the original.<br> **Requires an Elevenlabs API token.** <br> Does more processing than the others: noises are entirely eliminated, but it distort the soundstage to put the voice in the center. It might feel a bit uncanny in an enhanced track. |

> [!NOTE]
> demucs and spleeter are originally meant for _songs_ (ie. tracks a few minutes long) and the GPUs allocated by Replicate to these models are not the best. You may encounter _OOM GPU (out of memory) errors_ when trying to process audio tracks of movies. As far as my testing goes, trying a few hours later solves the problem.<br> Replicate also offers to make deployments with a GPU of one's choice, but this isn't cost-effective or user-friendly so it probably won't ever be supported.

### Parallelization / multi-threading by default
By default all CPU cores available are used. You can reduce CPU usage by passing a lower ```--workers``` value than the default.

### Bulk / recursive directory processing
...if you pass a directory instead of a mp4. The target and native language must be set using ```-l```, see tldr section.

## ...But why?
There are plenty of good options already: [Language Reactor](https://www.languagereactor.com/) (previously Language Learning With Netflix), [asbplayer](https://github.com/killergerbah/asbplayer), [mpvacious](https://github.com/Ajatt-Tools/mpvacious), [voracious](https://github.com/rsimmons/voracious), [memento](https://github.com/ripose-jp/Memento)...

Here is a list: [awesome-immersion](https://github.com/nakopylov/awesome-immersion)

They are awesome but all of them are media-centric: they are implemented around watching shows.

The approach here is word-centric:
- **word-centric notes referencing all common meanings**: I cross-source dictionaries, LLMs to the map the meanings, connotations and register of a word. Then I use another tool to search my database of generated TSV to illustrate & disambiguate with real-world examples the meanings I have found. This results in high quality notes regrouping all examples sentences, TTS, picture... and any other fields related to the word, allowing for maximum context.
- **word-note reuse for language laddering**: another advantage of this approach it that you can use this very note as basis for making cards for a new target language further down the line, while keeping all your previous note fields at hand for making the cards template for your new target language. The initial language acts just like Note ID for a meaning mapped across multiple languages. The majority of the basic vocabulary can be translated across languages directly with no real loss of meaning (and you can go on to disambiguate it further, using the method above for example). The effort that you spend on your first target language will thus pay off on subsequent languages.

There are several additional tools I made to accomplish this but they are hardcoded messes so don't expect me to publish them, langkit is enough work for me by itself! :)

### License
All new contributions from commit d540bd4 onward are licensed under **GPL-3.0**.

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

- [movies2anki](https://github.com/kelciour/movies2anki): Fully-integrated add-on for Anki which has some advanced features and supports all platforms
- [substudy](https://github.com/emk/subtitles-rs/tree/master/substudy): CLI alternative to subs2srs with the ability to export into other formats as well, not just SRS decks
- [subs2srs](http://subs2srs.sourceforge.net/): GUI software for Windows with many features, and inspiration for substudy and Bunkai
