# Langkit Addon Configuration

This addon integrates Langkit's language learning tools directly into Anki. Langkit helps you transform native media content (movies, TV shows, etc.) into language learning materials.

## Configuration Options

### binary_path

- **Type**: string or null
- **Default**: null
- **Description**: Path to the langkit executable. If null, the addon will auto-detect or download the appropriate binary for your platform.
- **Example**: "/path/to/langkit" or null

### download_timeout

- **Type**: integer
- **Default**: 300
- **Description**: Maximum time in seconds to wait for binary downloads. Increase this on slow connections.

### process_startup_timeout

- **Type**: integer
- **Default**: 30
- **Description**: Maximum time in seconds to wait for the Langkit server to start.

### config_poll_interval

- **Type**: float
- **Default**: 0.5
- **Description**: How often (in seconds) to check the config file for port information during startup.

### config_poll_timeout

- **Type**: integer
- **Default**: 10
- **Description**: Maximum time in seconds to wait for Langkit to write port information.

## Usage

### Opening Langkit

1. Click the "Langkit" button in Anki's main toolbar (between Stats and Sync)
2. Or use the Langkit menu in the menu bar

### First Time Setup

1. The addon will automatically download the appropriate Langkit binary for your platform
2. The download includes checksum verification for security
3. Once downloaded, the Langkit server will start automatically

### Server Management

- The Langkit server runs in the background as a subprocess
- It's automatically stopped when you close Anki
- You can manually control it via the Langkit menu:
  - Start Server
  - Stop Server
  - Restart Server

### Updates

- If auto_update is enabled, the addon checks for updates on startup
- You can manually check via Langkit → Check for Updates
- Updates are downloaded with progress indication and checksum verification

## Troubleshooting

### Server Won't Start

1. Check Langkit → Show Diagnostics for detailed information
2. Ensure no other program is using the required ports
3. Try Langkit → Restart Server

### Download Fails

1. Check your internet connection
2. Increase download_timeout if on a slow connection
3. Check if GitHub is accessible from your network
4. You can manually download from https://github.com/tassa-yoniso-manasi-karoto/langkit/releases

### WebView Issues

- The addon uses a single WebEngine instance to avoid memory issues
- If the interface appears blank, try restarting the server
- Enable developer tools with: `QTWEBENGINE_REMOTE_DEBUGGING=9222`

## Advanced Configuration

### Manual Binary Installation

If automatic download fails or you want to use a custom build:

1. Download the appropriate binary for your platform
2. Set "binary_path" in the config to the full path
3. Ensure the binary has execute permissions (Linux/macOS)

### Platform-Specific Notes

- **Windows**: The binary runs without a console window
- **macOS**: The .app bundle is automatically extracted from the zip
- **Linux**: Execute permissions are set automatically

## Data Storage

- Binaries are stored in: `[addon]/user_files/binaries/`
- This directory persists across addon updates
- Temporary config files are created in your system's temp directory

## Support

- Report issues: https://github.com/tassa-yoniso-manasi-karoto/langkit/issues
- Documentation: https://github.com/tassa-yoniso-manasi-karoto/langkit