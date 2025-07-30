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
        
        # Enable drag and drop
        self.setAcceptDrops(True)
    
    def dragEnterEvent(self, event: QDragEnterEvent) -> None:
        """Handle drag enter events."""
        if event.mimeData().hasUrls():
            event.acceptProposedAction()
            # Inject visual feedback into the page
            self.page().runJavaScript("""
                if (window.handleDragEnter) {
                    window.handleDragEnter();
                }
            """)
        else:
            event.ignore()
    
    def dragMoveEvent(self, event: QDragMoveEvent) -> None:
        """Handle drag move events."""
        if event.mimeData().hasUrls():
            event.acceptProposedAction()
        else:
            event.ignore()
    
    def dragLeaveEvent(self, event: QDragLeaveEvent) -> None:
        """Handle drag leave events."""
        event.accept()
        # Remove visual feedback
        self.page().runJavaScript("""
            if (window.handleDragLeave) {
                window.handleDragLeave();
            }
        """)
    
    def dropEvent(self, event: QDropEvent) -> None:
        """Handle drop events."""
        print("[Langkit] Drop event received")
        
        if event.mimeData().hasUrls():
            event.acceptProposedAction()
            
            # Remove visual feedback first
            self.page().runJavaScript("""
                if (window.handleDragLeave) {
                    window.handleDragLeave();
                }
            """)
            
            # Process the dropped files
            for url in event.mimeData().urls():
                file_path = url.toLocalFile()
                if file_path:  # Ensure it's a local file
                    print(f"[Langkit] Dropped file: {file_path}")
                    # Escape the file path for JavaScript
                    escaped_path = file_path.replace('\\', '\\\\').replace('"', '\\"')
                    # Inject the file drop into the page
                    self.page().runJavaScript(f"""
                        if (window.handleFileDrop) {{
                            window.handleFileDrop("{escaped_path}");
                        }} else {{
                            console.warn('[Langkit] window.handleFileDrop not found');
                        }}
                    """)
                    # Only handle the first file for now
                    break
        else:
            event.ignore()


class LangkitTab:
    """Manages the Langkit tab in Anki's main window."""
    
    def __init__(self, process_manager):
        self.process_manager = process_manager
        self.web_view: Optional[LangkitWebView] = None
        self.is_visible = False
        self.original_toolbar_height: Optional[int] = None
        self.original_bottom_height: Optional[int] = None
        
        # Store original methods for restoration
        self._original_toolbar_draw = None
        self._original_toolbar_redraw = None
        self._original_toolbar_adjustHeight = None
        self._original_bottomWeb_adjustHeight = None
        self._original_onRefreshTimer = None
        self._original_toolbar_show = None
        self._original_bottomWeb_show = None
        self._original_web_show = None
        self._original_web_setHtml = None
        self._original_web_stdHtml = None
        self._original_deckBrowser_refresh = None
        self._original_deckBrowser_refresh_if_needed = None
        
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
        
        # Disable Anki's auto-refresh mechanisms while Langkit is visible
        self._disable_anki_refresh()
        
        # Mark Langkit as visible on main window
        mw._langkit_visible = True
        
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
        
        # FIRST: Restore Anki's auto-refresh mechanisms (including show() methods)
        self._restore_anki_refresh()
        
        # THEN: Show and restore Anki's webviews with the restored methods
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
        
        # Mark Langkit as not visible on main window
        mw._langkit_visible = False
        
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
            
        # Ensure flag is cleared
        mw = aqt.mw
        if hasattr(mw, '_langkit_visible'):
            mw._langkit_visible = False
            
        if self.web_view:
            self.web_view.setUrl(QUrl("about:blank"))
            self.web_view.deleteLater()
            self.web_view = None
            
    def _disable_anki_refresh(self):
        """Temporarily disable Anki's auto-refresh mechanisms to prevent UI conflicts."""
        mw = aqt.mw
        
        # Store original methods
        if hasattr(mw.toolbar, 'draw'):
            self._original_toolbar_draw = mw.toolbar.draw
            self._original_toolbar_redraw = mw.toolbar.redraw
        
        if hasattr(mw.toolbarWeb, 'adjustHeightToFit'):
            self._original_toolbar_adjustHeight = mw.toolbarWeb.adjustHeightToFit
            
        if hasattr(mw.bottomWeb, 'adjustHeightToFit'):
            self._original_bottomWeb_adjustHeight = mw.bottomWeb.adjustHeightToFit
            
        if hasattr(mw, 'onRefreshTimer'):
            self._original_onRefreshTimer = mw.onRefreshTimer
            
        # Store show methods
        if hasattr(mw.toolbarWeb, 'show'):
            self._original_toolbar_show = mw.toolbarWeb.show
            
        if hasattr(mw.bottomWeb, 'show'):
            self._original_bottomWeb_show = mw.bottomWeb.show
            
        if hasattr(mw.web, 'show'):
            self._original_web_show = mw.web.show
            
        # Store setHtml and stdHtml methods
        if hasattr(mw.web, 'setHtml'):
            self._original_web_setHtml = mw.web.setHtml
            
        if hasattr(mw.web, 'stdHtml'):
            self._original_web_stdHtml = mw.web.stdHtml
            
        # Store deck browser refresh methods
        if hasattr(mw, 'deckBrowser'):
            if hasattr(mw.deckBrowser, 'refresh'):
                self._original_deckBrowser_refresh = mw.deckBrowser.refresh
            if hasattr(mw.deckBrowser, 'refresh_if_needed'):
                self._original_deckBrowser_refresh_if_needed = mw.deckBrowser.refresh_if_needed
        
        # Replace with no-op functions
        def noop(*args, **kwargs):
            print("[Langkit] Blocked Anki refresh attempt while Langkit is visible")
            pass
        
        # Disable toolbar operations
        if hasattr(mw.toolbar, 'draw'):
            mw.toolbar.draw = noop
            mw.toolbar.redraw = noop
            
        # Disable height adjustments
        if hasattr(mw.toolbarWeb, 'adjustHeightToFit'):
            mw.toolbarWeb.adjustHeightToFit = noop
            
        if hasattr(mw.bottomWeb, 'adjustHeightToFit'):
            mw.bottomWeb.adjustHeightToFit = noop
            
        # Disable refresh timer
        if hasattr(mw, 'onRefreshTimer'):
            mw.onRefreshTimer = noop
            
        # Disable show methods
        if hasattr(mw.toolbarWeb, 'show'):
            mw.toolbarWeb.show = noop
            
        if hasattr(mw.bottomWeb, 'show'):
            mw.bottomWeb.show = noop
            
        if hasattr(mw.web, 'show'):
            mw.web.show = noop
            
        # Replace setHtml to prevent automatic show()
        if hasattr(mw.web, 'setHtml'):
            def setHtml_no_show(html, context=None):
                print("[Langkit] Blocked setHtml which would trigger show()")
                # Don't call the original setHtml as it calls show()
                # The content update is blocked while Langkit is visible
                pass
            mw.web.setHtml = setHtml_no_show
            
        # Replace stdHtml to prevent setHtml call
        if hasattr(mw.web, 'stdHtml'):
            def stdHtml_no_show(*args, **kwargs):
                print("[Langkit] Blocked stdHtml which would trigger setHtml/show()")
                # Don't call the original stdHtml as it calls setHtml which calls show()
                pass
            mw.web.stdHtml = stdHtml_no_show
            
        # Disable deck browser refresh
        if hasattr(mw, 'deckBrowser'):
            if hasattr(mw.deckBrowser, 'refresh'):
                mw.deckBrowser.refresh = noop
            if hasattr(mw.deckBrowser, 'refresh_if_needed'):
                mw.deckBrowser.refresh_if_needed = noop
            
        print("[Langkit] Disabled Anki refresh mechanisms")
            
    def _restore_anki_refresh(self):
        """Restore Anki's original auto-refresh mechanisms."""
        mw = aqt.mw
        
        # Restore toolbar methods
        if self._original_toolbar_draw:
            mw.toolbar.draw = self._original_toolbar_draw
            self._original_toolbar_draw = None
            
        if self._original_toolbar_redraw:
            mw.toolbar.redraw = self._original_toolbar_redraw
            self._original_toolbar_redraw = None
            
        # Restore height adjustment methods
        if self._original_toolbar_adjustHeight:
            mw.toolbarWeb.adjustHeightToFit = self._original_toolbar_adjustHeight
            self._original_toolbar_adjustHeight = None
            
        if self._original_bottomWeb_adjustHeight:
            mw.bottomWeb.adjustHeightToFit = self._original_bottomWeb_adjustHeight
            self._original_bottomWeb_adjustHeight = None
            
        # Restore refresh timer
        if self._original_onRefreshTimer:
            mw.onRefreshTimer = self._original_onRefreshTimer
            self._original_onRefreshTimer = None
            
        # Restore show methods
        if self._original_toolbar_show:
            mw.toolbarWeb.show = self._original_toolbar_show
            self._original_toolbar_show = None
            
        if self._original_bottomWeb_show:
            mw.bottomWeb.show = self._original_bottomWeb_show
            self._original_bottomWeb_show = None
            
        if self._original_web_show:
            mw.web.show = self._original_web_show
            self._original_web_show = None
            
        # Restore setHtml and stdHtml
        if self._original_web_setHtml:
            mw.web.setHtml = self._original_web_setHtml
            self._original_web_setHtml = None
            
        if self._original_web_stdHtml:
            mw.web.stdHtml = self._original_web_stdHtml
            self._original_web_stdHtml = None
            
        # Restore deck browser refresh methods
        if hasattr(mw, 'deckBrowser'):
            if self._original_deckBrowser_refresh:
                mw.deckBrowser.refresh = self._original_deckBrowser_refresh
                self._original_deckBrowser_refresh = None
                
            if self._original_deckBrowser_refresh_if_needed:
                mw.deckBrowser.refresh_if_needed = self._original_deckBrowser_refresh_if_needed
                self._original_deckBrowser_refresh_if_needed = None
            
        print("[Langkit] Restored Anki refresh mechanisms")