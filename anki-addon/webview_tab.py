"""
WebView tab integration for Langkit addon.
Implements single instance pattern for Qt WebEngine to avoid memory issues.
"""

import os
from typing import Optional

import aqt
from aqt.qt import *
from aqt.utils import showWarning, showInfo


class LangkitWebPage(QWebEnginePage):
    """Custom WebPage with navigation control."""
    
    def acceptNavigationRequest(self, url: QUrl, 
                              nav_type: QWebEnginePage.NavigationType,
                              is_main_frame: bool) -> bool:
        """Handle navigation requests - open external URLs in system browser."""
        print(f"[Langkit] Navigation request: {url.toString()}, type: {nav_type}, mainFrame: {is_main_frame}")
        
        # Allow navigation to localhost (langkit server)
        if url.host() in ["localhost", "127.0.0.1", ""]:  # "" for initial setHtml
            print(f"[Langkit] Allowing localhost navigation")
            return True
            
        # Open external URLs in system browser
        if nav_type == QWebEnginePage.NavigationType.NavigationTypeLinkClicked:
            print(f"[Langkit] Opening external URL in browser: {url.toString()}")
            QDesktopServices.openUrl(url)
            return False
            
        # Allow other navigation types (redirects, form submissions, etc)
        print(f"[Langkit] Allowing other navigation type")
        return super().acceptNavigationRequest(url, nav_type, is_main_frame)


class LangkitWebView(QWebEngineView):
    """Custom WebView for Langkit, decoupled from AnkiWebView."""
    
    def __init__(self, parent=None):
        super().__init__(parent)
        
        # Create our custom page
        self._page = LangkitWebPage(self)
        self.setPage(self._page)
        
        # Configure WebEngine settings
        settings = self.settings()
        settings.setAttribute(QWebEngineSettings.WebAttribute.LocalContentCanAccessRemoteUrls, True)
        settings.setAttribute(QWebEngineSettings.WebAttribute.JavascriptEnabled, True)
        settings.setAttribute(QWebEngineSettings.WebAttribute.PluginsEnabled, False)
        settings.setAttribute(QWebEngineSettings.WebAttribute.LocalContentCanAccessFileUrls, True)
            
        # Set focus policy to ensure webview can receive input
        self.setFocusPolicy(Qt.FocusPolicy.StrongFocus)


class LangkitTab:
    """Manages the Langkit tab in Anki's main window."""
    
    def __init__(self, process_manager):
        self.process_manager = process_manager
        self.web_view: Optional[LangkitWebView] = None
        self.tab_widget: Optional[QTabWidget] = None
        self.tab_index: Optional[int] = None
        self.original_central_widget: Optional[QWidget] = None
        self.is_visible = False
        
        # Don't create webview here - wait until show() is called
        
    def _create_webview(self):
        """Create the WebEngineView instance."""
        self.web_view = LangkitWebView()
        self.web_view.setObjectName("langkitWebView")
        
        # Set a loading page initially
        self.web_view.setHtml("""
            <html>
            <head>
                <style>
                    body {
                        display: flex;
                        justify-content: center;
                        align-items: center;
                        height: 100vh;
                        margin: 0;
                        background: #f0f0f0;
                        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
                    }
                    .container {
                        text-align: center;
                        padding: 2em;
                        background: white;
                        border-radius: 8px;
                        box-shadow: 0 2px 10px rgba(0,0,0,0.1);
                    }
                    h1 { color: #333; margin-bottom: 0.5em; }
                    p { color: #666; }
                    .spinner {
                        margin: 20px auto;
                        width: 50px;
                        height: 50px;
                        border: 3px solid #f3f3f3;
                        border-top: 3px solid #3498db;
                        border-radius: 50%;
                        animation: spin 1s linear infinite;
                    }
                    @keyframes spin {
                        0% { transform: rotate(0deg); }
                        100% { transform: rotate(360deg); }
                    }
                </style>
            </head>
            <body>
                <div class="container">
                    <h1>Langkit</h1>
                    <div class="spinner"></div>
                    <p>Starting server...</p>
                </div>
            </body>
            </html>
        """)
        
        # Handle load finished
        self.web_view.loadFinished.connect(self._on_load_finished)
        
        
    def show(self):
        """Show the Langkit interface."""
        print(f"[Langkit] show() called, is_visible={self.is_visible}")
        if self.is_visible:
            return
            
        mw = aqt.mw
        
        # Start server if not running - BEFORE creating any UI
        if not self.process_manager.is_running():
            print("[Langkit] Server not running, attempting to start...")
            if not self.process_manager.start():
                # Server failed to start - don't create any UI
                print("[Langkit] Server failed to start, returning without creating UI")
                return  # Error message already shown by process_manager
                
        # Get the frontend URL
        url = self.process_manager.get_frontend_url()
        if not url:
            showWarning("Could not get Langkit server URL")
            return
            
        # Only NOW start creating the UI, after we know server is working
        print(f"[Langkit] Server is running, creating UI with URL: {url}")
        
        # Create webview if it doesn't exist yet (first time showing)
        if not self.web_view:
            print("[Langkit] Creating webview for the first time")
            self._create_webview()
        
        # Load the URL into webview
        self.web_view.setUrl(QUrl(url))
        
        # Store original central widget
        self.original_central_widget = mw.centralWidget()
        
        # Create a container with back button
        container = QWidget()
        layout = QVBoxLayout(container)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)
        
        # Create toolbar with back button
        toolbar = QToolBar()
        toolbar.setMovable(False)
        toolbar.setIconSize(QSize(16, 16))
        
        back_action = QAction("← Back to Anki", toolbar)
        back_action.triggered.connect(self.hide)
        toolbar.addAction(back_action)
        
        # Add some space
        spacer = QWidget()
        spacer.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Preferred)
        toolbar.addWidget(spacer)
        
        # Add toolbar and webview to container
        layout.addWidget(toolbar)
        layout.addWidget(self.web_view)
        
        # Replace central widget
        print("[Langkit] Replacing central widget - window should appear now")
        mw.setCentralWidget(container)
        
        self.is_visible = True
        print("[Langkit] UI creation complete")
        
    def hide(self):
        """Hide the Langkit interface and restore Anki."""
        if not self.is_visible:
            return
            
        mw = aqt.mw
        
        # Restore original central widget
        if self.original_central_widget:
            mw.setCentralWidget(self.original_central_widget)
            self.original_central_widget = None
            
        self.is_visible = False
        
        # The webview is kept alive but hidden (single instance pattern)
        
    def _on_load_finished(self, ok: bool):
        """Handle page load completion."""
        if not ok and self.is_visible and self.web_view:
            # Check if server is still running
            if not self.process_manager.is_running():
                showWarning("Langkit server has stopped. Attempting to restart...")
                if self.process_manager.restart():
                    # Reload the page
                    url = self.process_manager.get_frontend_url()
                    if url:
                        self.web_view.setUrl(QUrl(url))
                else:
                    self.hide()
                    showWarning("Failed to restart Langkit server")
                    
    def cleanup(self):
        """Clean up resources."""
        if self.is_visible:
            self.hide()
            
        if self.web_view:
            self.web_view.setUrl(QUrl("about:blank"))
            self.web_view.deleteLater()
            self.web_view = None