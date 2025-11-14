"""
Qt Dialog Handler for Langkit Anki Addon.
Provides HTTP endpoints for file dialogs that the Go backend can call.
"""

import json
import os
import socket
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Dict, Optional, Tuple
from urllib.parse import parse_qs, urlparse

from aqt.qt import *
from aqt import mw


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
            else:
                self.send_error(404, "Dialog endpoint not found")
                return

            # Send response
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(result).encode('utf-8'))

        except Exception as e:
            print(f"[Langkit Dialog] Error handling request: {e}")
            self.send_error(500, str(e))

    def do_GET(self):
        """Handle GET requests - used for health check."""
        if self.path == '/health':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok"}).encode('utf-8'))
        else:
            self.send_error(404, "Not found")


class DialogSignalHandler(QObject):
    """Qt signal handler for thread-safe dialog operations."""

    # Signals that can be emitted from any thread
    show_save_signal = pyqtSignal(dict)
    show_open_signal = pyqtSignal(dict)
    show_directory_signal = pyqtSignal(dict)

    def __init__(self, parent_window=None):
        super().__init__()
        self.parent_window = parent_window or mw
        self.result = None
        self.result_event = threading.Event()

        # Connect signals to slots (must be done in main thread)
        self.show_save_signal.connect(self._show_save_dialog)
        self.show_open_signal.connect(self._show_open_dialog)
        self.show_directory_signal.connect(self._show_directory_dialog)

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