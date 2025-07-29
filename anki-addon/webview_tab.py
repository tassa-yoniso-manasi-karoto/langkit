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
        self.is_visible = False
        self.original_toolbar_height: Optional[int] = None
        self.original_bottom_height: Optional[int] = None
        
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
        
        # Add ESC key shortcut to return to Anki
        escape_shortcut = QShortcut(QKeySequence("Escape"), self.web_view)
        escape_shortcut.activated.connect(self.hide)
        
        
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
        
        # Store original heights and hide Anki's webviews (push approach)
        print("[Langkit] Hiding Anki's webviews")
        
        # Store and collapse toolbar height
        self.original_toolbar_height = mw.toolbarWeb.height()
        print(f"[Langkit] Storing toolbar height: {self.original_toolbar_height}")
        mw.toolbarWeb.setFixedHeight(0)
        mw.toolbarWeb.hide()
        
        # Hide main webview (no height adjustment needed)
        mw.web.hide()
        
        # Store and collapse bottom bar height
        self.original_bottom_height = mw.bottomWeb.height()
        print(f"[Langkit] Storing bottom bar height: {self.original_bottom_height}")
        mw.bottomWeb.setFixedHeight(0)
        mw.bottomWeb.hide()
        
        # Add Langkit webview to the main layout
        print("[Langkit] Adding Langkit webview to main layout")
        mw.mainLayout.addWidget(self.web_view)
        
        self.is_visible = True
        print("[Langkit] UI creation complete")
        
    def hide(self):
        """Hide the Langkit interface and restore Anki."""
        if not self.is_visible:
            return
            
        mw = aqt.mw
        
        # Remove Langkit webview from the layout
        print("[Langkit] Removing Langkit webview from layout")
        mw.mainLayout.removeWidget(self.web_view)
        self.web_view.setParent(None)  # Detach from layout but keep alive
        
        # Show and restore Anki's webviews
        print("[Langkit] Showing Anki's webviews")
        
        # Restore toolbar
        mw.toolbarWeb.show()
        if self.original_toolbar_height:
            mw.toolbarWeb.setFixedHeight(self.original_toolbar_height)
        
        # Show main webview
        mw.web.show()
        
        # Restore bottom bar
        mw.bottomWeb.show()
        if self.original_bottom_height:
            mw.bottomWeb.setFixedHeight(self.original_bottom_height)
        
        # Redraw toolbar to ensure theme consistency
        if hasattr(mw, 'toolbar') and mw.toolbar:
            mw.toolbar.redraw()
            
        self.is_visible = False
        
        # The webview is kept alive but hidden (single instance pattern)
        
    def _on_load_finished(self, ok: bool):
        """Handle page load completion."""
        if ok and self.is_visible and self.web_view:
            # Set up global function for returning to Anki (without visual hint)
            js_code = """
            window.returnToAnki = function() {
                document.title = '__LANGKIT_RETURN_TO_ANKI__';
            };
            """
            self.web_view.page().runJavaScript(js_code)
            # Connect to title change signal
            self.web_view.titleChanged.connect(self._on_title_changed)
        elif not ok and self.is_visible and self.web_view:
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
                    
    def _on_title_changed(self, title: str):
        """Handle title changes as a communication channel."""
        if title == "__LANGKIT_RETURN_TO_ANKI__":
            print("[Langkit] Return to Anki requested via title change")
            self.hide()
                    
    def cleanup(self):
        """Clean up resources."""
        if self.is_visible:
            # Make sure to restore Anki's webviews before cleanup
            self.hide()
            
        if self.web_view:
            self.web_view.setUrl(QUrl("about:blank"))
            self.web_view.deleteLater()
            self.web_view = None