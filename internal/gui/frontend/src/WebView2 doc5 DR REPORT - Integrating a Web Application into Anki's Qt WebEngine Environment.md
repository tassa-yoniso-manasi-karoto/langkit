# Integrating a Web Application into Anki's Qt WebEngine Environment

## Assessment and technical feasibility

Based on comprehensive research, integrating your language processing companion app into Anki as an add-on is technically feasible with several viable approaches. The migration from Wails to standard web protocols aligns well with Anki's WebEngine architecture, though specific adaptations will be required for native features.

## Qt WebEngine capabilities in Anki

Anki's Qt WebEngine provides robust support for modern web standards essential to your application. The current implementation uses Qt 6.x with WebEngine based on Chromium 130+, offering full support for WebSocket APIs, fetch, ES6+ JavaScript, and Web Workers. The key limitation is that all JavaScript evaluation is asynchronous, requiring callback-based patterns rather than direct returns.

The Python-JavaScript bridge is implemented through Qt WebChannel, providing bidirectional communication via Anki's `pycmd()` function and the `onBridgeCmd()` handler. This architecture supports JSON serialization and asynchronous messaging patterns suitable for your WebRPC requirements.

## Recommended architecture approach

### Frontend integration strategy

Your Svelte/TypeScript webapp should be hosted in a full Qt WebEngine view rather than an iframe. This approach provides better performance, native feature access, and seamless integration with Anki's existing infrastructure. The webapp assets would be served through Anki's internal media server using the established pattern:

```python
from aqt import mw
from aqt.webview import AnkiWebView

class LanguageProcessorView(AnkiWebView):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.setup_bridge()
        self.load_webapp()
    
    def load_webapp(self):
        addon_dir = os.path.dirname(__file__)
        html_path = os.path.join(addon_dir, "web", "index.html")
        
        # Register web exports for asset serving
        mw.addonManager.setWebExports(__name__, r"web/.*(js|css|png|svg)")
        
        # Load through Anki's media server
        webview_id = id(self)
        mw.mediaServer.set_page_html(webview_id, html_content, context)
        self.load_url(QUrl(f"{mw.serverURL()}_anki/pages/{webview_id}.html"))
```

### Backend integration options

For your Go backend server, two primary approaches emerge:

**Option 1: Embedded Go binary (Recommended for performance-critical features)**
- Bundle platform-specific Go binaries within the add-on
- Use Python's subprocess module for process lifecycle management
- Implement dynamic port allocation to avoid conflicts
- Size optimization through Go build flags: `go build -ldflags="-s -w"`

**Option 2: Python replacement (Recommended for simpler deployment)**
- Replace Go backend with Python using asyncio and websockets
- Eliminates binary distribution complexity
- Provides adequate performance for most language processing tasks
- Easier debugging within Anki's Python environment

## Native feature bridging implementation

### File operations through Qt dialogs

```python
from PyQt5.QtCore import QObject, pyqtSlot, QVariant
from PyQt5.QtWidgets import QFileDialog

class NativeBridge(QObject):
    @pyqtSlot(str, str, result=QVariant)
    def open_file_dialog(self, title, filter_str):
        file_path, _ = QFileDialog.getOpenFileName(None, title, "", filter_str)
        return {
            "success": bool(file_path),
            "path": file_path
        }
    
    @pyqtSlot(str, result=QVariant)
    def select_directory(self, title):
        directory = QFileDialog.getExistingDirectory(None, title)
        return {
            "success": bool(directory),
            "path": directory
        }
```

### FFmpeg and MediaInfo execution

```python
import subprocess
from PyQt5.QtCore import QProcess, pyqtSignal

class MediaProcessor(QObject):
    progress_updated = pyqtSignal(int)
    
    @pyqtSlot(str, QVariant, result=QVariant)
    def process_media(self, input_file, options):
        try:
            # Use QProcess for better integration
            self.process = QProcess()
            self.process.setProcessChannelMode(QProcess.MergedChannels)
            
            cmd = ["ffmpeg", "-i", input_file]
            cmd.extend(options.get("args", []))
            
            self.process.start(cmd[0], cmd[1:])
            return {"success": True, "message": "Processing started"}
        except Exception as e:
            return {"success": False, "error": str(e)}
```

### JavaScript integration

```javascript
// WebChannel setup in your Svelte app
class AnkiBridge {
    constructor() {
        this.bridge = null;
        this.ready = false;
        
        new QWebChannel(qt.webChannelTransport, (channel) => {
            this.bridge = channel.objects.nativeBridge;
            this.ready = true;
            this.onReady();
        });
    }
    
    async selectFile(title, filter) {
        if (!this.ready) throw new Error('Bridge not ready');
        
        return new Promise((resolve, reject) => {
            this.bridge.open_file_dialog(title, filter, (result) => {
                if (result.success) {
                    resolve(result.path);
                } else {
                    reject(new Error('File selection cancelled'));
                }
            });
        });
    }
}
```

## Development workflow recommendations

### Project structure
```
language-processor-anki/
├── __init__.py           # Main add-on entry
├── manifest.json         # Add-on metadata
├── bridge/               # Native bridge implementations
│   ├── file_ops.py
│   ├── media_proc.py
│   └── process_mgr.py
├── web/                  # Svelte build output
│   ├── index.html
│   ├── bundle.js
│   └── bundle.css
├── server/               # Backend implementation
│   └── websocket.py      # Python WebSocket server
└── bin/                  # Optional Go binaries
    ├── server_windows_amd64.exe
    ├── server_darwin_amd64
    └── server_linux_amd64
```

### Debugging setup
Enable Qt WebEngine remote debugging:
```bash
export QTWEBENGINE_REMOTE_DEBUGGING=8080
```
Then access Chrome DevTools at `http://localhost:8080` for full debugging capabilities.

### Build automation
```python
# tasks.py for Invoke
from invoke import task

@task
def build_web(c):
    """Build Svelte application"""
    c.run("cd webapp && npm run build")
    c.run("cp -r webapp/dist/* src/web/")

@task
def package(c):
    """Create .ankiaddon package"""
    build_web(c)
    c.run("cd src && zip -r ../language-processor.ankiaddon .")
```

## Migration path implementation

### Phase 1: Core infrastructure (Weeks 1-2)
- Set up Qt WebChannel bridge for file operations
- Implement basic WebSocket server in Python
- Create minimal Svelte integration test

### Phase 2: Feature parity (Weeks 3-4)
- Port FFmpeg/MediaInfo execution to Qt subprocess
- Implement all file system operations
- Complete WebSocket API migration

### Phase 3: Optimization (Weeks 5-6)
- Performance testing and optimization
- Cross-platform testing (Windows/macOS/Linux)
- User experience refinement

## Common pitfalls and solutions

### WebEngine asynchronous operations
All JavaScript evaluation in Qt WebEngine is asynchronous. Replace synchronous patterns:
```python
# Wrong - WebKit style
result = webview.page().evaluateJavaScript("getState()")

# Correct - WebEngine style
def callback(result):
    process_result(result)
webview.page().runJavaScript("getState()", callback)
```

### Cross-platform binary management
If using Go binaries, implement platform detection:
```python
import platform
import sys

def get_server_binary():
    system = platform.system().lower()
    machine = platform.machine().lower()
    
    if system == "windows":
        return f"server_windows_{machine}.exe"
    elif system == "darwin":
        return f"server_darwin_{machine}"
    else:
        return f"server_linux_{machine}"
```

### Security considerations
Implement path validation for file operations:
```python
def is_safe_path(path):
    allowed_dirs = [
        os.path.expanduser("~/Documents/Anki"),
        mw.col.media.dir()
    ]
    abs_path = os.path.abspath(path)
    return any(abs_path.startswith(allowed) for allowed in allowed_dirs)
```

## Performance optimization strategies

### Asset loading optimization
- Bundle and minify JavaScript/CSS assets
- Use Qt resource system for small static files
- Implement lazy loading for large components

### Process management
- Start backend server on-demand rather than at startup
- Implement connection pooling for WebSocket connections
- Use QProcess for better integration with Qt event loop

### Memory considerations
- Qt WebEngine runs in separate processes, increasing memory usage
- Monitor and limit concurrent media processing operations
- Implement proper cleanup in add-on shutdown hooks

## Alternative: Hybrid approach

Consider maintaining both deployment modes initially:
```javascript
// Feature detection in Svelte app
const deployment = {
    isAnkiAddon: typeof window.pycmd !== 'undefined',
    hasNativeBridge: typeof window.ankiBridge !== 'undefined',
    
    async selectFile(title, filter) {
        if (this.hasNativeBridge) {
            return window.ankiBridge.selectFile(title, filter);
        } else {
            // Fallback to HTML file input
            return this.htmlFileSelect(title, filter);
        }
    }
};
```

This allows gradual migration while maintaining the standalone version for users who prefer it.

## Conclusion

Integrating your web application into Anki's Qt WebEngine is technically feasible and offers significant benefits for Anki users. The recommended approach leverages Anki's existing WebView infrastructure with Qt WebChannel for native feature bridging, while either embedding your Go backend as managed subprocess or replacing it with a Python implementation for simpler deployment. The key to success lies in embracing Anki's asynchronous JavaScript patterns, implementing robust error handling, and thoroughly testing across platforms before release.