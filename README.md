<p align="center">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-blackBg.webp">
        <source media="(prefers-color-scheme: light)" srcset="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
        <img width=375 src="https://github.com/tassa-yoniso-manasi-karoto/langkit/raw/refs/heads/master/internal/drawing-whiteBg.webp">
    </picture>
</p>


### Status: alpha

Langkit is an all-in-one tool designed to **facilitate language learning from native media content** using a collection of diverse features to transform movies, TV shows, etc., into **easily ‘digestible’ material**.

It supports automatic subtitle detection, bulk/recursive directory processing, seamless resumption of previously interrupted processing runs and multiple native (reference) language fallback.

### Features

- **Subs2cards**: Make Anki cards from subtitle timecodes like subs2srs
- **Making dubtitles¹**: Make a subtitle file of dubs using Speech-To-Text
- **Voice enhancing**: Make voices louder than Music&Effects 
- **Condensed Audio**: generate an abridged audio file containing only the dialogue from media for passive immersion (see explanation video linked below)
- **Subtitle romanization²**
- **Subtitle tokenization**: Separate words with spaces for languages which don't use spaces
- **Selective transliteration**: selective transliteration of subtitles based on [logogram](https://en.wikipedia.org/wiki/Logogram) frequency. Currently only japanese Kanjis are supported. Kanji with a frequency rank below the user-defined frequency threshold and regular readings are preserved, while others are converted to hiragana.

<sub> ¹ 'dubtitles' is a *subtitle file that matches the dubbing lines exactly*. It is needed because translations of dubbings and of subtitles differ, as explained [here](https://www.quora.com/Why-do-subtitles-on-a-lot-of-dubbed-shows-not-match-up-with-the-dub-itself)</sup>

<sup> ² for the list of supported languages by the transliteration feature see [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators) </sub>
<br>
<br>

> [!IMPORTANT]
> **Some features require an API key because certain processing tasks, such as speech-to-text, audio enhancement, are outsourced to an external provider** like Replicate. These companies offer cloud-based machine learning models that handle complex tasks remotely, allowing Langkit to leverage the models without requiring local computation. <br> The cost of running a few processing tasks using these models is typically very low or free. 

> [!WARNING]
> ⚠️ **About Feature Combinations**: ⚠️<br> langkit provides numerous features, some of which may overlap or influence each other's behavior, creating a complex network of conditional interactions. Although relatively extensive testing has been conducted, the multitude of possible combinations mean that certain specific scenarios *will* still contain bugs / unexpected behavior, especially when utilizing less common or more intricate feature combinations. Users are encouraged to **report any issues encountered either with the Debug Report exported from the Settings panel or with the Crash Report.**

# Langkit within Anki

Langkit can run as a standalone but it can now also run directly inside Anki. This offers better performance so **it's the recommended way to use Langkit.**

[Anki Addon available here](https://ankiweb.net/shared/info/1639192026)

# tldr cli

```
𝗕𝗮𝘀𝗶𝗰 𝘀𝘂𝗯𝘀𝟮𝘀𝗿𝘀 𝗳𝘂𝗻𝗰𝘁𝗶𝗼𝗻𝗮𝗹𝗶𝘁𝘆
$ langkit subs2cards media.mp4 media.th.srt media.en.srt

𝗕𝘂𝗹𝗸 𝗽𝗿𝗼𝗰𝗲𝘀𝘀𝗶𝗻𝗴 𝘄𝗶𝘁𝗵 𝗮𝘂𝘁𝗼𝗺𝗮𝘁𝗶𝗰 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘀𝗲𝗹𝗲𝗰𝘁𝗶𝗼𝗻 (𝘩𝘦𝘳𝘦: 𝘭𝘦𝘢𝘳𝘯 𝘣𝘳𝘢𝘻𝘪𝘭𝘪𝘢𝘯 𝘱𝘰𝘳𝘵𝘶𝘨𝘦𝘴𝘦 𝘧𝘳𝘰𝘮 𝘤𝘢𝘯𝘵𝘰𝘯𝘦𝘴𝘦 𝘰𝘳 𝘵𝘳𝘢𝘥𝘪𝘵𝘪𝘰𝘯𝘢𝘭 𝘤𝘩𝘪𝘯𝘦𝘴𝘦)
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

> [!WARNING]
> The focus of my recent work has been the GUI therefore the CLI has been much less tested and is **much more unstable at this point**. Some features are not yet supported on the CLI.

# Features in detail...

# Subs2cards
Subs2cards converts your favorite TV shows and movies directly into Anki flashcards by extracting dialogues, images, and audio clips based on subtitle timecodes. It's ideal for  sentence mining and context-aware word memorization. 
    
####  Extra features compared to subs2srs

- **Default encoding to OPUS / AVIF**: Use modern codecs to save storage.
- **Parallelization / multi-threading by default**: By default all CPU cores available are used. You can reduce CPU usage by passing a lower ```--workers``` value than the default.
- **Bulk / recursive directory processing**: if you pass a directory instead of a mp4. The target and native language must be set using ```-l```, see tldr section.
- **Seamless resumption of previously interrupted runs**
- **Dubtitles as source of truth of for subtitle lines** (when both are selected together)

# Condensed Audio
langkit can **make an audio file containing all the audio snippets of dialog** in the audiotrack. <br>
This is meant to be used for passive listening. <br>
👉 *Explanations and context here:* [Optimizing Passive Immersion: Condensed Audio - YouTube](https://www.youtube.com/watch?v=QOLTeO-uCYU)

Additionally a **summary of the episode/media can be generated by an AI (LLM)** in your native language and embeded in the audiofile to refresh your memories before listening and help your understand the condensed audio's content. Openrouter serves a number of LLMs that can do this **for free** (Deepseek, Llama, Qwen...). 

# Voice Enhancing

**Boosts clarity of speech in audio tracks by amplifying voices while reducing background music and effects.** Ideal for learners who struggle with distinguishing words clearly, particularly useful for tonal languages or when studying languages with dense or unfamiliar phonetic patterns.

This feature works by merging the original audiotrack with negative gain together with an audiotrack containing the voices only with additional gain. Obtaining the isolated voice track requires running using one of these deep learning audio separation tool in the cloud:

| Name (to be passed with --sep) | Quality of separated vocals | Price                               | Type        | Note                                                                                                                                 |
|--------------------------------|-----------------------------|-------------------------------------|-------------|--------------------------------------------------------------------------------------------------------------------------------------|
| demucs, de                     | good                        | very cheap 0.063$/run               | MIT license | **Recommended**                                                                                                            |
| demucs_ft, ft                  | good                        | cheap 0.252$/run                    | MIT license | <sub>Fine-tuned version: "take 4 times more time but might be a bit better".</sub> <br> I couldn't hear any difference with the original in my test. | 
| spleeter, sp                   | rather poor                 | very, very cheap 0.00027$/run       | MIT license |                                                                                                                                      |
| elevenlabs, 11, el             | good                        | very, very expensive<br>1$/*MINUTE* | proprietary | Not supported on the GUI. Not fully supported on the CLI <br> <sub>due to limitations of their API (mp3 only) which desync the processed audio with the original. Does more processing than the others: noises are entirely eliminated, but it distort the soundstage to put the voice in the center. It might feel a bit uncanny in an enhanced track.</sub> |



# Dubtitles
Creates accurate subtitle files specifically synchronized with dubbed audio tracks using speech-to-text. This addresses the common mismatch between subtitle translations and audio dubs, ensuring text follows closely spoken dialogue.

> [!IMPORTANT]
> **Don't rely on the Word Error Rate (WER) for all languages, check for *your* specific target language!**


| Provider   | Name (to be passed with --stt) | Num of lang supported (WER<50%)                                                               | WER (ALL languages) (lower is better)                                                           | WER per language                                                               | Price         | Type   |
|------------|--------------------------------|-----------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|---------------|--------|
| OpenAI     | gpt-4o-transcribe              | [57](https://platform.openai.com/docs/guides/speech-to-text/supported-languages%5C)           | [8,9%](https://artificialanalysis.ai/speech-to-text)                                            | [here](https://openai.com/index/introducing-our-next-generation-audio-models/) | $6/1000min    | closed |
| OpenAI     | gpt-4o-mini-transcribe         | [57](https://platform.openai.com/docs/guides/speech-to-text/supported-languages%5C)           | [13,9%]( https://artificialanalysis.ai/speech-to-text)                                          | [here](https://openai.com/index/introducing-our-next-generation-audio-models/) | $3/1000min    | closed |
| Elevenlabs | scribe                         | [99](https://elevenlabs.io/speech-to-text#:~:text=What%20languages%20does%20Scribe%20support) | [7,7%](https://web.archive.org/web/20250317011655/https://artificialanalysis.ai/speech-to-text) | [here](https://pbs.twimg.com/media/GschVxrWMAALJon?format=png&name=4096x4096)  | $6.67/1000min | closed |
| Replicate  | whisper, wh                    | [57](https://platform.openai.com/docs/guides/speech-to-text/supported-languages%5C)           | 10,3%                                                                                           | [here](https://github.com/openai/whisper/discussions/1762)                     | $1.1/1000min  | MIT    |
| Replicate  | insanely-fast-whisper, fast    | 57?                                                                                           | 16,2%                                                                                           | n/a                                                                            | $0.0071/run   | MIT    |



# Subtitle romanization
Convert subtitles into a roman character version as phonetically accurate as possible


The list of supported languages by the transliteration feature is [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators)


# Subtitle tokenization

Separate words with spaces for languages which don't use spaces

The list of supported languages by the tokenization feature is [here](https://github.com/tassa-yoniso-manasi-karoto/translitkit?tab=readme-ov-file#currently-implemented-tokenizers--transliterators)

# Selective (Kanji) Transliteration

Automatically transliterates Japanese subtitles from kanji into hiragana based on user-defined frequency thresholds and phonetic regularity. This feature helps Japanese learners focus on common kanji by selectively converting rarer or irregular characters into easier-to-read hiragana, facilitating incremental kanji learning and smoother immersion into native content.

The frequency list comes from 6th edition of "Remembering the Kanji" by James W. Heisig and supports the most 3000 frequent Kanjis.


# FAQ
<details>
<summary> 
    
#### On Windows I get a blue popup that says "Windows protected your PC" error when trying to run Langkit.
</summary>

When running for the first time, Windows may show "Windows protected your PC":
1. Click "More info" 
2. Click "Run anyway"

This is **normal for unsigned software that isn't widely used** and should only happen once.

For a more technical explanation: This error is triggered by Windows Defender SmartScreen, a security feature that protects against unknown applications. Langkit is flagged because it  is not a widely-used application, it has not been seen by Microsoft's telemetry systems and therefore has no history of being safe.

</details>

<details>
<summary> 
    
#### How do I get these API keys?
</summary>
API keys are only visible once during creation.

### Replicate
- **Navigation:** Click your username (top left) → API Tokens → Create token
- **URL:** https://replicate.com/account/api-tokens  
- Limited free credits for new users. After free credits, pay-as-you-go starting at $0.000100/second for CPU.

### OpenRouter
- **Navigation:** Login → Keys → Create API Key → Name Key → Create
- **URL:** https://openrouter.ai  
- Small free allowance for testing. Several models offer free variants marked with `:free`.

### OpenAI
- **Navigation:** Dashboard → API Keys (left menu under "Organization") → Create new secret key → Name key → Generate  
- **URL:** https://platform.openai.com  
- No free credits for new accounts. Phone verification required. Must purchase credits before API usage.

### Google AI
- **Navigation:** Dashboard (top right) → Create an API key
- **URLs:** https://aistudio.google.com  
- Generious free tier. No credit card required for free access but rated limited.

### ElevenLabs
- **Navigation:** Profile (bottom left) → My Account → API Keys → Create  
- **URL:** https://elevenlabs.io/app/settings/api-keys  
- 10,000 free credits monthly.

</details>

<details>

<summary> 

#### Why isn't there the possibility to run the speech-to-text or voice separation locally?
</summary>

Because I only have a 10 year old Pentium CPU with a graphic chipset.
</details>

<details>
<summary> 

#### Why is the executable/binary so heavy ?
</summary>

The official Docker + Docker Compose libraries and their dependencies make up most of the size of the executable.
</details>


# Download

See [Releases](https://github.com/tassa-yoniso-manasi-karoto/langkit/releases)

# Requirements
- FFmpeg **v6 or higher (dev builds being preferred)**,
  - The FFmpeg dev team recommends end-users to use only the latest [builds from the dev branch (master builds)](https://github.com/BtbN/FFmpeg-Builds/releases). 
- [MediaInfo](https://mediaarea.net/en/MediaInfo/Download),
- *(optional)* [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Windows/MacOS) / Docker Engine (Linux): only if you need to process subtitles in Japanese or any Indic languages

The binary's location for FFmpeg and Mediainfo can be provided by a flag, in $PATH or in a "bin" directory placed in the folder where langkit is. 


Using static FFmpeg builds guarantee that you have up-to-date codecs. **If you don't use a well-maintained bleeding edge distro or brew, use the dev builds.** You can check your distro [here](https://repology.org/project/ffmpeg/versions).

## API Keys

Certain features, like voice enhancement and speech-to-text, require API keys from external cloud services. You can provide these keys using the GUI (recommended) or via environment variables for CLI-only usage.

### Method 1: GUI Settings Panel (Recommended)

The easiest way to configure your keys is through the **Settings** panel in the Langkit application. Simply paste your keys into the corresponding fields and click "Save Changes".

> [!IMPORTANT]
> Keys entered in the GUI are stored in a **plain text (unencrypted)** configuration file on your system. While convenient, this is less secure than using the environment variable method below and this file should be deleted in case you are using a public computer. <br>
> The configuration file is located at:
> -   **Windows:** `%APPDATA%\langkit\config.yaml` (use Notepad++ to open it)
> -   **Linux:** `~/.config/langkit/config.yaml`
> -   **macOS:** `~/Library/Application Support/langkit/config.yaml`

### Method 2: Environment Variables (no API key persistence)

For CLI users or those who prefer not to store keys in a file, you can use environment variables. Set them in your shell (or in config file) before running Langkit.

| Service      | Environment Variable     |
| :----------- | :----------------------- |
| Replicate    | `REPLICATE_API_KEY`      |
| ElevenLabs   | `ELEVENLABS_API_KEY`     |
| OpenAI       | `OPENAI_API_KEY`         |
| OpenRouter   | `OPENROUTER_API_KEY`     |
| Google AI    | `GOOGLE_API_KEY`         |

### How Keys are Handled (Precedence and Saving)

1.  **Environment variables always take precedence.** If an environment variable is set, its value will be used for processing, even if a different key is saved in the GUI's configuration file.
2.  **The GUI is designed to protect your keys.** When you open the settings panel, it will load and display keys from your environment variables. However, to avoid writing secrets from your environment to disk, it will only save a key to the configuration file if you **explicitly paste a new value** into an API key field in the GUI.
3.  **The CLI does not write to the configuration file.** It will read and use keys from environment variables or the config file but will never save them.
4.  **Exported crash/debug reports are sanitized.** They are guaranteed not to leak any API keys.

# Output

(section may be outdated)

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
 
-   Special thanks to [Matt vs Japan](https://www.youtube.com/@mattvsjapan) for his excellent video essays on language acquisition.

### Linguistic Tools & Data

-   The core `subs2cards` functionality was first pioneered by **cb4960** with the original [subs2srs](https://subs2srs.sourceforge.net/) project.
-   Langkit began as a direct fork of [Bunkai](https://github.com/ustuehler/bunkai)) by ustuehler, which reimplemented `subs2srs` in Go.
-   Japanese morphological analysis is provided by the [ichiran](https://github.com/tshatrov/ichiran) project.
-   Indic scripts transliteration relies on the comprehensive [Aksharamukha](https://github.com/virtualvinodh/aksharamukha) script converter.
-   Thai transliteration is made possible by the [go-rod](https://github.com/go-rod/rod) library for browser automation and [thai2english](https://www.thai2english.com/) website

### Technical

**This project stands on the shoulders of giants and would not be possible without numerous open-source projects' contributions:**

-   Containerized linguistic analysis is managed with [Docker Compose](https://docs.docker.com/compose/).
-   Essential media processing depends on the indispensable [FFmpeg](https://ffmpeg.org/) and [MediaInfo](https://mediaarea.net/en/MediaInfo) tools.
-   The graphical user interface is:
    -   powered by [Wails](https://github.com/wailsapp/wails) web UI framework,
    -   built using the [Svelte](https://svelte.dev/) framework and styled using [Tailwind CSS](https://tailwindcss.com/).
- Shout out to the excellent [pyglossary](https://github.com/ilius/pyglossary) dictionary files conversion tool which inspired me to create a log viewer inside the GUI as well

# License
All new contributions from commit d540bd4 onward are licensed under **GPL-3.0**.

# Support the project
