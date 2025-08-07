# Langkit Anki Addon

This addon integrates Langkit into Anki, providing language learning tools directly within your flashcard workflow.

## Installation

### For Development/Testing

1. Ensure Anki 2.1.55+ is installed
2. Locate your Anki addons folder:
   - Windows: `%APPDATA%\Anki2\addons21\`
   - macOS: `~/Library/Application Support/Anki2/addons21/`
   - Linux: `~/.local/share/Anki2/addons21/`
3. Create a symbolic link or copy this `anki-addon` directory to the addons folder
4. Rename the folder to something like `langkit` (no hyphens allowed)
5. Restart Anki

### For Distribution

1. Package the addon:
   
   ```bash
   cd anki-addon
   zip -r langkit.ankiaddon * -x "meta.json" -x "__pycache__/*" -x "user_files/binaries/langkit*"
   ```

2. Users can install by:
   
   - Double-clicking the .ankiaddon file
   - Or via Tools → Add-ons → Install from file

## Testing Checklist

### Initial Setup

- [x] Addon loads without errors on Anki startup
- [x] Langkit button appears in toolbar between Stats and Sync
- [x] Langkit menu appears in menu bar

### Binary Management

- [x] First run triggers automatic binary download
- [x] Progress dialog shows during download
- [x] Checksum verification passes
- [x] Binary is saved to `user_files/binaries/`
- [x] Correct platform binary is selected

### Server Lifecycle

- [x] Server starts when clicking Langkit button
- [x] Temporary  `/tmp/langkit_addon_xxxxx.json` is created
- [x] Server writes port information to `/tmp/langkit_addon_xxxxx.json`
- [x] Frontend loads at dynamic port

### WebView Integration

- [x] WebView shows loading screen initially
- [x] Langkit frontend loads successfully
- [x] External links open in system browser
- [x] Back button returns to Anki

### Process Management

- [ ] Server stops when closing Anki
- [ ] Manual stop/start/restart work correctly
- [ ] Server recovers from crashes
- [ ] No zombie processes left behind

### Updates

- [x] Update check detects new versions
- [x] Update prompt shows version comparison
- [x] Binary update completes successfully
- [ ] Old binary is backed up during update

### Error Handling

- [ ] Network errors show user-friendly messages
- [x] Missing binary prompts for download
- [x] Port conflicts are detected
- [x] Diagnostics show helpful information

## Debug Mode

Enable Qt WebEngine developer tools:

```bash
QTWEBENGINE_REMOTE_DEBUGGING=9222 anki
```

Then visit http://localhost:9222 in Chrome/Chromium to inspect the WebView.

## Known Issues

1. **Memory Usage**: Qt WebEngine can use significant memory. The single instance pattern helps mitigate this.

2. **Firewall Warnings**: On first run, your firewall may prompt about langkit listening on localhost. This is safe to allow.

3. **macOS App Translocation**: If the langkit binary shows security warnings, you may need to manually approve it in System Preferences → Security & Privacy.

## Architecture Notes

- Uses single WebEngine instance pattern to avoid memory exhaustion
- Server runs as subprocess with --server flag
- Port discovery via config.json polling (not log parsing)
- Zenity dialogs used by langkit in server mode
- All business logic remains in Go backend

## File Structure

```
langkit_addon/
├── __init__.py           # Main addon entry point
├── binary_manager.py     # GitHub releases integration
├── process_manager.py    # Subprocess lifecycle
├── webview_tab.py       # Qt WebEngine UI
├── manifest.json        # Addon metadata
├── config.json          # User configuration
├── config.md            # Configuration docs
└── user_files/          # Persistent storage
    ├── README.txt
    └── binaries/        # Downloaded executables
```