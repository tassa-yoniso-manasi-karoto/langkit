<p align="center">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-blackBg.webp">
        <source media="(prefers-color-scheme: light)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
        <img width=375 src="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
    </picture>
</p>


### Status: prerelease

Langkit is an all-in-one tool designed to **facilitate language learning from native media content** using a collection of diverse features to transform movies, TV shows, etc., into **easily ‘digestible’ material**. It was made with scalability, fault-tolerance in mind and supports automatic subtitle detection, bulk/recursive directory processing, seamless resumption of previously interrupted processing runs and multiple native (reference) language fallback.

### Features

- **Subs2cards**: Make Anki cards from subtitle timecodes like subs2srs
- **Making dubtitles¹**: Make a subtitle file of dubs using Speech-To-Text
- **Voice enhancing**: Make voices louder than Music&Effects 
- **Subtitle romanization²**
- **Subtitle tokenization**: Separate words with spaces for languages which don't use spaces
- **Selective transliteration**: selective transliteration of subtitles based on [logogram](https://en.wikipedia.org/wiki/Logogram) frequency. Currently only japanese Kanjis are supported. Kanji with a frequency rank below the user-defined frequency threshold and regular readings are preserved, while others are converted to hiragana.

<sub> ¹ 'dubtitles' is a *subtitle file that matches the dubbing lines exactly*. It is needed because translations of dubbings and of subtitles differ, as explained [here](https://www.quora.com/Why-do-subtitles-on-a-lot-of-dubbed-shows-not-match-up-with-the-dub-itself)</sup>

<sup> ² for the list of supported languages by the transliteration feature see [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators) </sub>
<br>
<br>

> [!NOTE]
> **Some features require an API key because certain processing tasks, such as speech-to-text, audio enhancement, are outsourced to an external provider** like Replicate. These companies offer cloud-based machine learning models that handle complex tasks remotely, allowing Langkit to leverage the models without requiring local computation. <br> The cost of running a few processing tasks using these models is typically very low. 

> [!WARNING]
> ⚠️ **about Feature Combinations**: langkit provides numerous features, some of which may overlap or influence each other's behavior, creating a complex network of conditional interactions. Although relatively extensive testing has been conducted, the multitude of possible combinations mean that certain specific scenarios *will* still contain bugs / unexpected behavior. Users are encouraged to **report any issues encountered either with the debug info exported from the Setting panel or with the crash report log**, especially when utilizing less common or more intricate feature combinations.

# tldr cli

```
𝗕𝗮𝘀𝗶𝗰 𝘀𝘂𝗯𝘀𝟮𝘀𝗿𝘀 𝗳𝘂𝗻𝗰𝘁𝗶𝗼𝗻𝗮𝗹𝗶𝘁𝘆
$ langkit subs2cards media.mp4 media.th.srt media.en.srt

𝗕𝘂𝗹𝗸 𝗽𝗿𝗼𝗰𝗲𝘀𝘀𝗶𝗻𝗴 𝘄𝗶𝘁𝗵 𝗮𝘂𝘁𝗼𝗺𝗮𝘁𝗶𝗰 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘀𝗲𝗹𝗲𝗰𝘁𝗶𝗼𝗻 (𝘩𝘦𝘳𝘦: 𝘭𝘦𝘢𝘳𝘯 𝘣𝘳𝘢𝘻𝘪𝘭𝘪𝘢𝘯 𝘱𝘰𝘳𝘵𝘶𝘨𝘦𝘴𝘦 𝘧𝘳𝘰𝘮 𝘤𝘢𝘯𝘵𝘰𝘯𝘦𝘴𝘦 𝘰𝘳 𝘪𝘧 𝘶𝘯𝘢𝘷𝘢𝘪𝘭𝘢𝘣𝘭𝘦, 𝘵𝘳𝘢𝘥𝘪𝘵𝘪𝘰𝘯𝘢𝘭 𝘤𝘩𝘪𝘯𝘦𝘴𝘦)
$ langkit subs2cards media.mp4 -l "pt-BR,yue,zh-Hant"

𝗦𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘁𝗿𝗮𝗻𝘀𝗹𝗶𝘁𝗲𝗿𝗮𝘁𝗶𝗼𝗻 (+𝘁𝗼𝗸𝗲𝗻𝗶𝘇𝗮𝘁𝗶𝗼𝗻 𝗶𝗳 𝗻𝗲𝗰𝗲𝘀𝘀𝗮𝗿𝘆)
$ langkit translit media.ja.srt

𝗠𝗮𝗸𝗲 𝗮𝗻 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝘄𝗶𝘁𝗵 𝗲𝗻𝗵𝗮𝗻𝗰𝗲𝗱/𝗮𝗺𝗽𝗹𝗶𝗳𝗶𝗲𝗱 𝘃𝗼𝗶𝗰𝗲𝘀 𝗳𝗿𝗼𝗺 𝘁𝗵𝗲 𝟮𝗻𝗱 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝗼𝗳 𝘁𝗵𝗲 𝗺𝗲𝗱𝗶𝗮 (𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘯𝘦𝘦𝘥𝘦𝘥)
$ langkit enhance media.mp4 -a 2 --sep demucs

𝗠𝗮𝗸𝗲 𝗱𝘂𝗯𝘁𝗶𝘁𝗹𝗲𝘀 𝘂𝘀𝗶𝗻𝗴 𝗦𝗽𝗲𝗲𝗰𝗵-𝘁𝗼-𝗧𝗲𝘅𝘁 𝗼𝗻 𝘁𝗵𝗲 𝘁𝗶𝗺𝗲𝗰𝗼𝗱𝗲𝘀 𝗼𝗳 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗱 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝗳𝗶𝗹𝗲 (𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘯𝘦𝘦𝘥𝘦𝘥)
$ langkit subs2dubs --stt whisper media.mp4 (media.th.srt) -l "th"

𝗖𝗼𝗺𝗯𝗶𝗻𝗲 𝗮𝗹𝗹 𝗼𝗳 𝘁𝗵𝗲 𝗮𝗯𝗼𝘃𝗲 𝗶𝗻 𝗼𝗻𝗲 𝗰𝗼𝗺𝗺𝗮𝗻𝗱
$ langkit subs2cards /path/to/media/dir/  -l "th,en" --stt whisper --sep demucs --translit
```

# Features in detail

## Subs2cards
Subs2cards converts your favorite TV shows and movies directly into Anki flashcards by extracting dialogues, images, and audio clips based on subtitle timecodes. It's ideal for  sentence mining and context-aware word memorization. 

<details>
<summary> 
    
#### Details
</summary>

#### Extra features compared to subs2srs

- **Default encoding to OPUS / AVIF**: Use modern codecs to save storage.
- **Parallelization / multi-threading by default**: By default all CPU cores available are used. You can reduce CPU usage by passing a lower ```--workers``` value than the default.
- **Bulk / recursive directory processing**: if you pass a directory instead of a mp4. The target and native language must be set using ```-l```, see tldr section.
- **Seamless resumption of previously interrupted runs**

</details>

## Dubtitles
Creates accurate subtitle files specifically synchronized with dubbed audio tracks using speech-to-text. This addresses the common mismatch between subtitle translations and audio dubs, ensuring text follows closely spoken dialogue.

<details>
<summary> 
    
#### Details
</summary>

By default **a dubtitle file will also be created from these transcriptions.**

AFAIK Language Reactor was the first to combine this with language learning from content however I found the accuracy of the STT they use to be unimpressive.
    
| Name (to be passed with --stt) | Word Error Rate average across all supported langs (june 2024) | Number of languages supported        | Price        | Type        | Note                                                                                                                                                                                                                                                                                                                   |
|--------------------------------|-----------------|-------------------------------------------------------------------------------------|--------------|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| whisper, wh                    | 10,3%           | [57](https://platform.openai.com/docs/guides/speech-to-text/supported-languages%5C) | $1.1/1000min | MIT         | **See [here](https://github.com/openai/whisper/discussions/1762) for a breakdown of WER per language.**  |
| insanely-fast-whisper, fast    | 16,2%           | 57                                                                                  | $0.0071/run  | MIT         |                                                                                                                                                                                                                                                                                                                        |
| universal-1, u1                | 8,7%            | [17](https://www.assemblyai.com/docs/getting-started/supported-languages)           | $6.2/1000min | proprietary | **Untested** (doesn't support my target lang)                                                                                                                                                                                                                                                                          |

See  [ArtificialAnalysis](https://artificialanalysis.ai/speech-to-text) and [Amgadoz @Reddit](https://www.reddit.com/r/LocalLLaMA/comments/1brqwun/i_compared_the_different_open_source_whisper/) for detailed comparisons.

Note: OpenAI just released a [turbo](https://github.com/openai/whisper/discussions/1762) model of large-v3 but they say it's on a par with large-v2 as far as accuracy is concerned so I won't bother to add it.


</details>

## Voice Enhancing

Boosts clarity of speech in audio tracks by amplifying voices while reducing background music and effects. Ideal for learners who struggle with distinguishing words clearly, particularly useful for tonal languages or when studying languages with dense or unfamiliar phonetic patterns.

<details>
<summary> 
    
#### Details
</summary>
This feature works by merging the original audiotrack with negative gain together with an audiotrack containing the voices only with additional gain.

The isolated voice track are obtained using one of these:

| Name (to be passed with --sep) | Quality of separated vocals | Price                               | Type        | Note                                                                                                                                 |
|--------------------------------|-----------------------------|-------------------------------------|-------------|--------------------------------------------------------------------------------------------------------------------------------------|
| demucs, de                     | good                        | very cheap 0.063$/run               | MIT license | **Recommended**                                                                                                            |
| demucs_ft, ft                  | good                        | cheap 0.252$/run                    | MIT license | Fine-tuned version: "take 4 times more time but might be a bit better". I couldn't hear any difference with the original in my test. | 
| spleeter, sp                   | rather poor                 | very, very cheap 0.00027$/run       | MIT license |                                                                                                                                      |
| elevenlabs, 11, el             | good                        | very, very expensive<br>1$/*MINUTE* | proprietary | Not supported on the GUI. Not fully supported on the CLI due to limitations of their API (mp3 only) which desync the processed audio with the original.<br> **Requires an Elevenlabs API token.** <br> Does more processing than the others: noises are entirely eliminated, but it distort the soundstage to put the voice in the center. It might feel a bit uncanny in an enhanced track. |

</details>

## Subtitle romanization
Convert subtitles into a roman character version as phonetically accurate as possible

<details>
<summary> 
    
#### Details
</summary>
The list of supported languages by the transliteration feature is [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators)
</details>

## Subtitle tokenization

Separate words with spaces for languages which don't use spaces

<details>
<summary> 
    
#### Details
</summary>
The list of supported languages by the tokenization feature is [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators)
</details>


## Selective (Kanji) Transliteration

Automatically transliterates Japanese subtitles from kanji into hiragana based on user-defined frequency thresholds and phonetic regularity. This feature helps Japanese learners focus on common kanji by selectively converting rarer or irregular characters into easier-to-read hiragana, facilitating incremental kanji learning and smoother immersion into native content.

<details>
<summary> 
    
#### Details
</summary>
The frequency list comes from 6th edition of "Remembering the Kanji" by James W. Heisig and supports the most 3000 frequent Kanjis.
</details>


<!-- ## (⚠️fixme⚠️) Condensed Audio
langkit will automatically **make an audio file containing all the audio snippets of dialog** in the audiotrack. <br>
This is meant to be used for passive listening. <br>
More explanations and context here: [Optimizing Passive Immersion: Condensed Audio - YouTube](https://www.youtube.com/watch?v=QOLTeO-uCYU) -->


# Requirements
This fork require FFmpeg **v6 or higher (dev builds being preferred)**, Mediainfo, a [Replicate](https://replicate.com/home) API token.

The FFmpeg dev team recommends end-users to use only the latest [builds from the dev branch (master builds)](https://github.com/BtbN/FFmpeg-Builds/releases). The FFmpeg binary's location can be provided by a flag, in $PATH or in a "bin" directory placed in the folder where langkit is.

The static FFmpeg builds guarantee that you have up-to-date codecs. **If you don't use a well-maintained bleeding edge distro or brew, use the dev builds.** You can check your distro [here](https://repology.org/project/ffmpeg/versions).

## API Keys
At the moment tokens should be passed through these env variables: REPLICATE_API_TOKEN, ASSEMBLYAI_API_KEY, ELEVENLABS_API_TOKEN.

$$
\color{red}
\text{TODO}
$$


# FAQ

<details>
<summary> 

#### Why isn't there the possibility to run the speech-to-text or voice separation locally?
</summary>

Because I only have a 10 year old Pentium CPU with a graphic chipset.
</details>
<details>
<summary> 
    
#### Why use subs2srs approach nowdays?
</summary>

There are plenty of alternative comprehensible input companion already: [Language Reactor](https://www.languagereactor.com/) (previously Language Learning With Netflix), [asbplayer](https://github.com/killergerbah/asbplayer), [mpvacious](https://github.com/Ajatt-Tools/mpvacious), [voracious](https://github.com/rsimmons/voracious), [memento](https://github.com/ripose-jp/Memento)...

They are awesome but all of them are media-centric: they are implemented around watching shows.

The approach I take for card-making here is language-centric:
- **word-centric notes referencing all common meanings**: I cross-source dictionaries, LLMs to map the meanings, connotations, register of a word. Searching my database of generated TSV I can illustrate & disambiguate with real-world examples the meanings I have found. This results in high quality notes regrouping all examples sentences, TTS, picture... and any other fields related to the word, allowing for maximum context.
- **word-note reuse for language laddering**: another advantage of this approach it that you can use this very note as basis for making cards for a new target language further down the line, while keeping all your previous note fields at hand for making the cards template for your new target language. The initial language acts just like Note ID for a meaning mapped across multiple languages. The majority of the basic vocabulary can be translated across languages directly with no real loss of meaning (and you can go on to disambiguate it further, using the method above for example). The effort that you spend on your first target language will thus pay off on subsequent languages.

There are several additional tools I made to accomplish this but they are hardcoded messes and not meant to be published.

**Future developement could entirely automate the process of making cards described above using LLMs.**
</details>

## Output

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

# Build & Development

See [DEV.md](https://github.com/tassa-yoniso-manasi-karoto/langkit/blob/master/DEV.md)

# Aknowledgements
Fork of Bunkai, which reimplemented the functionality first pioneered by **cb4960** with [subs2srs](https://subs2srs.sourceforge.net/).

$$
\color{red}
\text{TODO}
$$




# License
All new contributions from commit d540bd4 onward are licensed under **GPL-3.0**.
