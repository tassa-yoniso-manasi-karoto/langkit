"""
IPC Server for Langkit Anki Addon.
Provides HTTP endpoints for file dialogs and system information queries
that the Go backend can call.
"""

import json
import os
import platform
import socket
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from typing import Dict, Optional, Tuple
from urllib.parse import parse_qs, urlparse

from aqt.qt import *
from aqt import mw
from aqt.utils import version_with_build


class DialogRequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler for dialog operations."""

    def log_message(self, format, *args):
        """Override to suppress default logging except for errors."""
        if args[1] != '200':
            print(f"[Langkit Dialog] {format % args}")

    def do_POST(self):
        """Handle POST requests for dialog operations."""
        try:
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)
            request_data = json.loads(post_data.decode('utf-8'))

            # Get the dialog handler from the server
            dialog_handler = self.server.dialog_handler

            # Parse path and execute appropriate dialog
            if self.path == '/dialog/save':
                result = dialog_handler.show_save_dialog(request_data)
            elif self.path == '/dialog/open':
                result = dialog_handler.show_open_dialog(request_data)
            elif self.path == '/dialog/directory':
                result = dialog_handler.show_directory_dialog(request_data)
            elif self.path == '/dialog/message':
                result = dialog_handler.show_message_dialog(request_data)
            else:
                self.send_error(404, "Dialog endpoint not found")
                return

            # Send response
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(result).encode('utf-8'))

        except BrokenPipeError:
            # Client closed connection before response was sent - this is normal
            pass
        except Exception as e:
            print(f"[Langkit Dialog] Error handling request: {e}")
            try:
                self.send_error(500, str(e))
            except BrokenPipeError:
                pass

    def do_GET(self):
        """Handle GET requests for health check and system info."""
        try:
            if self.path == '/health':
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({"status": "ok"}).encode('utf-8'))
            elif self.path == '/system-info':
                try:
                    info = get_anki_system_info()
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.end_headers()
                    self.wfile.write(json.dumps(info).encode('utf-8'))
                except Exception as e:
                    print(f"[Langkit IPC] Error getting system info: {e}")
                    self.send_error(500, str(e))
            else:
                self.send_error(404, "Not found")
        except BrokenPipeError:
            # Client closed connection before response was sent - this is normal
            pass


class DialogSignalHandler(QObject):
    """Qt signal handler for thread-safe dialog operations."""

    # Signals that can be emitted from any thread
    show_save_signal = pyqtSignal(dict)
    show_open_signal = pyqtSignal(dict)
    show_directory_signal = pyqtSignal(dict)
    show_message_signal = pyqtSignal(dict)

    def __init__(self, parent_window=None):
        super().__init__()
        self.parent_window = parent_window or mw
        self.result = None
        self.result_event = threading.Event()

        # Connect signals to slots (must be done in main thread)
        self.show_save_signal.connect(self._show_save_dialog)
        self.show_open_signal.connect(self._show_open_dialog)
        self.show_directory_signal.connect(self._show_directory_dialog)
        self.show_message_signal.connect(self._show_message_dialog)

    def _show_save_dialog(self, options: Dict):
        """Show save dialog (runs in main thread)."""
        try:
            title = options.get('title', 'Save File')
            default_filename = options.get('defaultFilename', '')
            filters_data = options.get('filters', [])

            # Build filter string
            filter_strings = []
            for f in filters_data:
                display_name = f.get('displayName', 'Files')
                patterns = f.get('pattern', '*.*').replace(';', ' ')
                filter_strings.append(f"{display_name} ({patterns})")

            filter_str = ';;'.join(filter_strings) if filter_strings else "All Files (*.*)"

            # Show dialog (this runs in main thread)
            path, _ = QFileDialog.getSaveFileName(
                self.parent_window,
                title,
                default_filename,
                filter_str
            )

            self.result = {'path': path or '', 'error': None}
        except Exception as e:
            self.result = {'path': '', 'error': str(e)}
        finally:
            self.result_event.set()

    def _show_open_dialog(self, options: Dict):
        """Show open dialog (runs in main thread)."""
        try:
            title = options.get('title', 'Open File')
            filters_data = options.get('filters', [])

            # Build filter string
            filter_strings = []
            for f in filters_data:
                display_name = f.get('displayName', 'Files')
                patterns = f.get('pattern', '*.*').replace(';', ' ')
                filter_strings.append(f"{display_name} ({patterns})")

            filter_str = ';;'.join(filter_strings) if filter_strings else "All Files (*.*)"

            # Show dialog
            path, _ = QFileDialog.getOpenFileName(
                self.parent_window,
                title,
                '',
                filter_str
            )

            self.result = {'path': path or '', 'error': None}
        except Exception as e:
            self.result = {'path': '', 'error': str(e)}
        finally:
            self.result_event.set()

    def _show_directory_dialog(self, options: Dict):
        """Show directory dialog (runs in main thread)."""
        try:
            title = options.get('title', 'Select Directory')

            # Show dialog
            path = QFileDialog.getExistingDirectory(
                self.parent_window,
                title,
                ''
            )

            self.result = {'path': path or '', 'error': None}
        except Exception as e:
            self.result = {'path': '', 'error': str(e)}
        finally:
            self.result_event.set()

    def _show_message_dialog(self, options: Dict):
        """Show message dialog (runs in main thread)."""
        try:
            title = options.get('title', 'Message')
            message = options.get('message', '')
            msg_type = options.get('type', 'info')

            # Map type to QMessageBox icon and buttons
            if msg_type == 'warning':
                icon = QMessageBox.Icon.Warning
                buttons = QMessageBox.StandardButton.Ok
            elif msg_type == 'error':
                icon = QMessageBox.Icon.Critical
                buttons = QMessageBox.StandardButton.Ok
            elif msg_type == 'question':
                icon = QMessageBox.Icon.Question
                buttons = QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No
            else:  # info
                icon = QMessageBox.Icon.Information
                buttons = QMessageBox.StandardButton.Ok

            # Show dialog
            msg_box = QMessageBox(self.parent_window)
            msg_box.setWindowTitle(title)
            msg_box.setText(message)
            msg_box.setIcon(icon)
            msg_box.setStandardButtons(buttons)

            result = msg_box.exec()

            # Determine if accepted (OK or Yes clicked)
            accepted = result in (
                QMessageBox.StandardButton.Ok,
                QMessageBox.StandardButton.Yes
            )

            self.result = {'accepted': accepted, 'error': None}
        except Exception as e:
            self.result = {'accepted': False, 'error': str(e)}
        finally:
            self.result_event.set()


class DialogHandler:
    """Handles Qt dialog operations for the Go backend."""

    def __init__(self, parent_window=None):
        self.parent_window = parent_window or mw
        self.server = None
        self.server_thread = None
        self.port = None

        # Create signal handler in main thread
        self.signal_handler = DialogSignalHandler(parent_window)

    def start_server(self) -> Optional[int]:
        """Start the HTTP server on a random available port."""
        try:
            # Find an available port
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.bind(('localhost', 0))
                s.listen(1)
                self.port = s.getsockname()[1]

            # Create HTTP server
            self.server = HTTPServer(('localhost', self.port), DialogRequestHandler)
            # Store reference to dialog handler in the server
            self.server.dialog_handler = self

            # Start server in background thread
            self.server_thread = threading.Thread(target=self.server.serve_forever, daemon=True)
            self.server_thread.start()

            print(f"[Langkit Dialog] Dialog server started on port {self.port}")
            return self.port

        except Exception as e:
            print(f"[Langkit Dialog] Failed to start dialog server: {e}")
            return None

    def stop_server(self):
        """Stop the HTTP server."""
        if self.server:
            try:
                self.server.shutdown()
                self.server = None
                self.server_thread = None
                print("[Langkit Dialog] Dialog server stopped")
            except Exception as e:
                print(f"[Langkit Dialog] Error stopping dialog server: {e}")

    def show_save_dialog(self, options: Dict) -> Dict:
        """Show a save file dialog and return the selected path."""
        try:
            # Reset result event
            self.signal_handler.result_event.clear()
            self.signal_handler.result = None

            # Emit signal to show dialog in main thread
            self.signal_handler.show_save_signal.emit(options)

            # Wait for dialog to complete (with timeout)
            if self.signal_handler.result_event.wait(timeout=120):
                return self.signal_handler.result
            else:
                return {'path': '', 'error': 'Dialog timeout'}
        except Exception as e:
            return {'path': '', 'error': str(e)}

    def show_open_dialog(self, options: Dict) -> Dict:
        """Show an open file dialog and return the selected path."""
        try:
            # Reset result event
            self.signal_handler.result_event.clear()
            self.signal_handler.result = None

            # Emit signal to show dialog in main thread
            self.signal_handler.show_open_signal.emit(options)

            # Wait for dialog to complete
            if self.signal_handler.result_event.wait(timeout=120):
                return self.signal_handler.result
            else:
                return {'path': '', 'error': 'Dialog timeout'}
        except Exception as e:
            return {'path': '', 'error': str(e)}

    def show_directory_dialog(self, options: Dict) -> Dict:
        """Show a directory selection dialog and return the selected path."""
        try:
            # Reset result event
            self.signal_handler.result_event.clear()
            self.signal_handler.result = None

            # Emit signal to show dialog in main thread
            self.signal_handler.show_directory_signal.emit(options)

            # Wait for dialog to complete
            if self.signal_handler.result_event.wait(timeout=120):
                return self.signal_handler.result
            else:
                return {'path': '', 'error': 'Dialog timeout'}
        except Exception as e:
            return {'path': '', 'error': str(e)}

    def show_message_dialog(self, options: Dict) -> Dict:
        """Show a message dialog and return whether it was accepted."""
        try:
            # Reset result event
            self.signal_handler.result_event.clear()
            self.signal_handler.result = None

            # Emit signal to show dialog in main thread
            self.signal_handler.show_message_signal.emit(options)

            # Wait for dialog to complete
            if self.signal_handler.result_event.wait(timeout=120):
                return self.signal_handler.result
            else:
                return {'accepted': False, 'error': 'Dialog timeout'}
        except Exception as e:
            return {'accepted': False, 'error': str(e)}


def get_anki_system_info() -> Dict:
    """Gather Anki system information for debug reports."""
    info = {
        "anki_version": "",
        "video_driver": "",
        "qt_version": "",
        "pyqt_version": "",
        "python_version": "",
        "platform": "",
        "langkit_addon_version": "",
        "screen": {
            "resolution": "",
            "refresh_rate": 0.0
        },
        "addons": {
            "active": [],
            "inactive": []
        }
    }

    try:
        # Anki version
        info["anki_version"] = version_with_build()
    except Exception as e:
        info["anki_version"] = f"error: {e}"

    try:
        # Video driver
        if mw and mw.pm:
            driver = mw.pm.video_driver()
            info["video_driver"] = driver.name if hasattr(driver, 'name') else str(driver)
    except Exception as e:
        info["video_driver"] = f"error: {e}"

    try:
        # Qt and PyQt versions
        info["qt_version"] = qVersion()
        info["pyqt_version"] = PYQT_VERSION_STR
    except Exception as e:
        info["qt_version"] = f"error: {e}"

    try:
        # Python version and platform
        info["python_version"] = platform.python_version()
        info["platform"] = platform.platform()
    except Exception as e:
        info["platform"] = f"error: {e}"


    try:
        # Langkit addon version from manifest.json
        addon_dir = Path(__file__).parent
        manifest_path = addon_dir / "manifest.json"
        if manifest_path.exists():
            with open(manifest_path, 'r', encoding='utf-8') as f:
                manifest = json.load(f)
                info["langkit_addon_version"] = manifest.get("human_version", "unknown")
        else:
            info["langkit_addon_version"] = "manifest not found"
    except Exception as e:
        info["langkit_addon_version"] = f"error: {e}"

    try:
        # Screen resolution and refresh rate
        screen = QGuiApplication.primaryScreen()
        if screen:
            geom = screen.geometry()
            info["screen"]["resolution"] = f"{geom.width()}x{geom.height()}"
            info["screen"]["refresh_rate"] = screen.refreshRate()
    except Exception as e:
        info["screen"]["resolution"] = f"error: {e}"

    try:
        # Addon list
        if mw and mw.addonManager:
            for addon in mw.addonManager.all_addon_meta():
                addon_id = addon.dir_name
                if addon.enabled:
                    info["addons"]["active"].append(addon_id)
                else:
                    info["addons"]["inactive"].append(addon_id)
    except Exception as e:
        info["addons"]["active"] = [f"error: {e}"]

    return info