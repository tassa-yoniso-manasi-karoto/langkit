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
make generate-go
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
