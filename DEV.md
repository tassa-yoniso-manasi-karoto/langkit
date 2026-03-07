# Target Versions

⚠️ Go 1.23.11 (Go Toolchain 1.23.11) <br>
⚠️ Wails CLI 2.9.0 <br>
Wails modules latest v2 <br>

> [!NOTE]
> These are the most recent version supported by [wails-action](https://github.com/dAppServer/wails-build-action), newer version will fail the build process due to CGo conflicts.

> [!WARNING]
> When contributing you **must use go 1.23** for `go mod tidy` otherwise the toolchain will be overwritten to a newer version. <br> <br>
> In other words, even if go1.23 is specified as go version, the **GH action will use the version specified by the toolchain for the build process** and thus it will fail. Use go version manager github.com/voidint/g to stay at 1.23 or correct manually the go.mod.


# Frontend Dependencies

⚠️ **Svelte 5.19.2** (exact version required) <br>
  🠲 use **pnpm** with `--frozen-lockfile` 

> [!WARNING]
> The frontend uses Svelte 4 patterns that break in Svelte 5.20+. Do NOT upgrade Svelte beyond 5.19.2.
> - Svelte 5.19: Last version with working Svelte 4 compatibility
> - Svelte 5.25: Requires extensive code refactoring (reactive variables)
> - Svelte 5.36+: Breaks even with refactoring
>
> See `internal/gui/frontend/src/BUG_REPORT_Svelte_Reactivity_Production_Builds.md` for details.

 Any version mismatch will cause feature cards' messages to not be displayed.

# Building from Source

### Prerequisites
1. **Go 1.23** (exactly - use [g](https://github.com/voidint/g) for version management)
2. **Node.js 18+** and **pnpm 10+**
3. **[Wails CLI](https://wails.io/docs/gettingstarted/installation/)**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
4. **Rust toolchain** (for WASM): `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`
5. **wasm32 target**: `rustup target add wasm32-unknown-unknown`
6. **System deps**:
   - Linux: `libgtk-3-dev libwebkit2gtk-4.0-dev`
   - Windows: WebView2 (pre-installed on modern win. versions)
   - macOS: Xcode Command Line Tools

## Build Steps

```bash
# Clone repository
git clone https://github.com/tassa-yoniso-manasi-karoto/langkit.git
cd langkit

# Generate API code
cd api
make generate all
cd ..

# Fetch dependancies
cd internal/gui/frontend
pnpm install --frozen-lockfile
cd ../../..

# Build desktop app
wails build

# Output: ./build/bin/langkit[.exe]
```

## Development Mode
```bash
wails dev
```

# Architecture Overview

Langkit uses a hybrid architecture designed for maximum flexibility.

This design allows the same codebase to adapt to different UI runtimes without modification while avoiding bugs of specific webviews (well, just WebView2 bugs really).

## Binary Modes
The application compiles to a single binary that operates in two modes:
- **GUI mode** (default): Full Wails-based desktop application
- **Server mode** (`--server` flag): Headless HTTP/WebSocket server for Qt WebEngine integration (Anki)

## Communication Architecture
The frontend-backend communication has been migrated away from Wails-specific APIs:
- **WebRPC over HTTP**: Type-safe RPC for all request/response operations (replaced Wails App methods)
- **WebSocket**: Real-time event broadcasting (logs, progress, state changes)
- **DOM injection**: API/WebSocket ports are injected into the HTML via `window.__LANGKIT_CONFIG__`

## UI Abstraction Layer
The `internal/ui` package provides a singleton-based abstraction for runtime-specific operations:
- **FileDialog interface**: Abstracts file/folder selection dialogs
- **Wails mode**: Uses native Wails dialogs (Windows/macOS/Linux integrated)
- **Server/Qt mode**: Uses [ncruces/zenity](github.com/ncruces/zenity) for cross-platform native dialogs, avoiding the need for Python IPC

## Key Architectural Decisions
- **No Wails dependencies in services**: All services use interfaces, enabling server mode to bypass Wails entirely while reusing its production-ready code wherever possible
- **Embedded frontend**: The Svelte frontend is embedded by wails in the binary using Go's `embed` package
- **Runtime detection**: The binary detects its mode at startup and initializes the appropriate UI provider
- **WebSocket service interface**: The `WebsocketService` interface (with `Emit` method) decouples services from the concrete WebSocket implementation

# Project stats

```bash
cloc . --include-ext=go,ts,svelte,rs,py,css --exclude-dir=node_modules,vendor,dist,build --not-match-f="\.gen\.(go|ts)\$" --not-match-f="(kanjis\.go|static\.go|deep_copy\.go)" --by-file-by-lang
```

# CLI

> [!WARNING]
> Unfortunately the CLI is de facto abandoned because I don't have the time nor the interest to maintain it anymore.

```
𝗕𝗮𝘀𝗶𝗰 𝘀𝘂𝗯𝘀𝟮𝘀𝗿𝘀 𝗳𝘂𝗻𝗰𝘁𝗶𝗼𝗻𝗮𝗹𝗶𝘁𝘆
$ langkit subs2cards media.mp4 media.th.srt media.en.srt

𝗕𝘂𝗹𝗸 𝗽𝗿𝗼𝗰𝗲𝘀𝘀𝗶𝗻𝗴 𝘄𝗶𝘁𝗵 𝗮𝘂𝘁𝗼𝗺𝗮𝘁𝗶𝗰 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘀𝗲𝗹𝗲𝗰𝘁𝗶𝗼𝗻 (𝘩𝘦𝘳𝘦: 𝘭𝘦𝘢𝘳𝘯 𝘣𝘳𝘢𝘻𝘪𝘭𝘪𝘢𝘯 𝘱𝘰𝘳𝘵𝘶𝘨𝘦𝘴𝘦 𝘧𝘳𝘰𝘮 𝘤𝘢𝘯𝘵𝘰𝘯𝘦𝘴𝘦 𝘰𝘳 𝘵𝘳𝘢𝘥𝘪𝘵𝘪𝘰𝘯𝘢𝘭 𝘤𝘩𝘪𝘯𝘦𝘴𝘦)
$ langkit subs2cards media.mp4 -l "pt-BR,yue,zh-Hant"

𝗦𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘁𝗿𝗮𝗻𝘀𝗹𝗶𝘁𝗲𝗿𝗮𝘁𝗶𝗼𝗻 (+𝘁𝗼𝗸𝗲𝗻𝗶𝘇𝗮𝘁𝗶𝗼𝗻 𝗶𝗳 𝗻𝗲𝗰𝗲𝘀𝘀𝗮𝗿𝘆)
$ langkit translit media.ja.srt

𝗠𝗮𝗸𝗲 𝗮𝗻 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝘄𝗶𝘁𝗵 𝗲𝗻𝗵𝗮𝗻𝗰𝗲𝗱/𝗮𝗺𝗽𝗹𝗶𝗳𝗶𝗲𝗱 𝘃𝗼𝗶𝗰𝗲𝘀 𝗳𝗿𝗼𝗺 𝘁𝗵𝗲 𝟮𝗻𝗱 𝗮𝘂𝗱𝗶𝗼𝘁𝗿𝗮𝗰𝗸 𝗼𝗳 𝘁𝗵𝗲 𝗺𝗲𝗱𝗶𝗮 (𝘋𝘰𝘤𝘬𝘦𝘳 𝘳𝘦𝘲𝘶𝘪𝘳𝘦𝘥 𝘧𝘰𝘳 𝘭𝘰𝘤𝘢𝘭, 𝘰𝘳 𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘧𝘰𝘳 𝘤𝘭𝘰𝘶𝘥)
$ langkit enhance media.mp4 -a 2 --sep docker-demucs

𝗠𝗮𝗸𝗲 𝗱𝘂𝗯𝘁𝗶𝘁𝗹𝗲𝘀 𝗳𝗿𝗼𝗺 𝗮𝗻 𝗲𝘅𝗶𝘀𝘁𝗶𝗻𝗴 𝗿𝗲𝗳𝗲𝗿𝗲𝗻𝗰𝗲 𝘀𝘂𝗯𝘁𝗶𝘁𝗹𝗲 𝘂𝘀𝗶𝗻𝗴 𝗦𝗽𝗲𝗲𝗰𝗵-𝘁𝗼-𝗧𝗲𝘅𝘁 (𝘙𝘦𝘱𝘭𝘪𝘤𝘢𝘵𝘦 𝘈𝘗𝘐 𝘵𝘰𝘬𝘦𝘯 𝘯𝘦𝘦𝘥𝘦𝘥)
$ langkit subs2dubs --stt whisper media.mp4 reference.en.srt -l "th"

𝗖𝗼𝗺𝗯𝗶𝗻𝗲 𝗮𝗹𝗹 𝗼𝗳 𝘁𝗵𝗲 𝗮𝗯𝗼𝘃𝗲 𝗶𝗻 𝗼𝗻𝗲 𝗰𝗼𝗺𝗺𝗮𝗻𝗱
$ langkit subs2cards /path/to/media/dir/  -l "th,en" --stt whisper --sep docker-demucs --translit
```

# Feature(s) selection to internal mode matrix

Feature selection must be 'translated' into a Task mode. These modes for the most part correspond to CLI subcommands.

<table><thead>
  <tr>
    <th>requires...</th>
    <th>sub?</th>
    <th>lang?</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make a merged video</td>
    <td>NO</td>
    <td>NO</td>
  </tr>
  <tr>
    <td>Make enhanced track</td>
    <td>NO</td>
    <td>opt</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>yes</td>
    <td>rather</td>
  </tr>
  <tr>
    <td>Make tokenized subtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make translit subtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make tokenized dubtitles</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make translit dubtitles</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make dubtitles</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>yes</td>
    <td>rather</td>
  </tr>
</tbody>
</table>

✅ = default behavior

🔳 = optionally available

❌ = not available

🚫 = not applicable

<table><thead>
  <tr>
    <th><sub>↓ GUI selected</sub>   ╲       <sup>tsk.Mode →</sup></th>
    <th>subs2cards</th>
    <th>subs2dubs</th>
    <th>translit</th>
    <th>condense</th>
    <th>enhance</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make a merged video</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
  </tr>
  <tr>
    <td>Make enhanced track</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>✅</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳<br></td>
    <td>✅</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized subtitle</td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make translit subtitle</td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅<br></td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized dubtitles</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>❌</td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make translit dubtitles</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>❌</td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make dubtitles</td>
    <td>🔳</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
</tbody></table>
