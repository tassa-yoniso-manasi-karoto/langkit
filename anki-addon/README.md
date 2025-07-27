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
4. Rename the folder to something like `langkit_addon` (no hyphens allowed)
5. Restart Anki

### For Distribution

1. Package the addon:
   ```bash
   cd anki-addon
   zip -r langkit.ankiaddon * -x "*.pyc" -x "__pycache__/*" -x ".git/*"
   ```
2. Users can install by:
   - Double-clicking the .ankiaddon file
   - Or via Tools → Add-ons → Install from file

## Testing Checklist

### Initial Setup
- [ ] Addon loads without errors on Anki startup
- [ ] Langkit button appears in toolbar between Stats and Sync
- [ ] Langkit menu appears in menu bar

### Binary Management
- [ ] First run triggers automatic binary download
- [ ] Progress dialog shows during download
- [ ] Checksum verification passes
- [ ] Binary is saved to `user_files/binaries/`
- [ ] Correct platform binary is selected

### Server Lifecycle
- [ ] Server starts when clicking Langkit button
- [ ] Temporary config.json is created
- [ ] Server writes port information to config.json
- [ ] Frontend loads at dynamic port

### WebView Integration
- [ ] WebView shows loading screen initially
- [ ] Langkit frontend loads successfully
- [ ] External links open in system browser
- [ ] Back button returns to Anki

### Process Management
- [ ] Server stops when closing Anki
- [ ] Manual stop/start/restart work correctly
- [ ] Server recovers from crashes
- [ ] No zombie processes left behind

### Updates
- [ ] Update check detects new versions
- [ ] Update prompt shows version comparison
- [ ] Binary update completes successfully
- [ ] Old binary is backed up during update

### Error Handling
- [ ] Network errors show user-friendly messages
- [ ] Missing binary prompts for download
- [ ] Port conflicts are detected
- [ ] Diagnostics show helpful information

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