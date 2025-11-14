"""
Langkit Addon for Anki
Integrates Langkit's language learning tools directly into Anki.
"""

import os
import sys
from pathlib import Path
from typing import Optional

import aqt
from aqt import mw, gui_hooks
from aqt.qt import *
from aqt.utils import showInfo, showWarning, qconnect
from aqt.utils import tr
from aqt.profiles import VideoDriver
from anki.utils import is_win, is_mac, is_lin
import aqt.toolbar

# Add addon directory to path for imports
addon_dir = os.path.dirname(__file__)
if addon_dir not in sys.path:
    sys.path.insert(0, addon_dir)

from binary_manager import BinaryManager
from process_manager import ProcessManager
from webview_tab import LangkitTab


class LangkitAddon:
    """Main addon controller."""
    
    def __init__(self):
        self.addon_path = Path(__file__).parent
        self.config = mw.addonManager.getConfig(__name__)
        self.binary_manager: Optional[BinaryManager] = None
        self.process_manager: Optional[ProcessManager] = None
        self.webview_tab: Optional[LangkitTab] = None
        self.update_checked_this_session = False
        self._toolbar_hook_callback = None  # Store the toolbar hook callback
        self._keyboard_shortcut = None  # Store the keyboard shortcut
        self._original_moveToState = None  # Store original moveToState method

        # Save config on changes
        mw.addonManager.setConfigAction(__name__, self._on_config_changed)
        
    def initialize(self):
        """Initialize the addon after profile is loaded."""
        # Create binary manager
        self.binary_manager = BinaryManager(self.addon_path, self.config)
        
        # Don't create process manager yet - wait until binary is available
        # Don't create webview tab yet - it needs process manager
        
        # Setup toolbar button
        self._setup_toolbar()

        # Setup keyboard shortcut
        self._setup_keyboard_shortcut()

        # Fix deck shortcut to work with Langkit
        self._fix_deck_shortcut()

        # Add menu items – for devs only
        # self._setup_menu()

        # Start on startup if configured (only if binary exists)
        if self.config.get("launch_on_startup", False) and self.binary_manager.check_binary_exists():
            QTimer.singleShot(1000, self._start_server)
            
    def _setup_toolbar(self):
        """Setup toolbar button using Anki's hook system."""
        # Store the callback so we can remove it later
        self._toolbar_hook_callback = self._on_toolbar_init
        gui_hooks.top_toolbar_did_init_links.append(self._toolbar_hook_callback)

        # Force toolbar redraw to show our button
        if hasattr(mw, 'toolbar') and mw.toolbar:
            mw.toolbar.draw()
        
    def _on_toolbar_init(self, links: list[str], toolbar: aqt.toolbar.Toolbar) -> None:
        """Add Langkit button to the main toolbar."""
        # Check if Langkit button already exists to prevent duplicates
        for link in links:
            if 'id="langkit"' in link:
                return  # Button already exists, don't add another

        langkit_link = toolbar.create_link(
            cmd="langkit",
            label="Langkit",
            func=self._on_open_langkit,
            tip=tr.actions_shortcut_key(val="L"),
            id="langkit"
        )
        # Insert before the last item (Sync) to place it between Stats and Sync
        if links:
            links.insert(-1, langkit_link)
        else:
            links.append(langkit_link)

    def _setup_keyboard_shortcut(self):
        """Setup keyboard shortcut for Langkit."""
        # Create the keyboard shortcut directly (lowercase 'l' like other Anki shortcuts)
        self._keyboard_shortcut = QShortcut(QKeySequence("l"), mw)
        self._keyboard_shortcut.activated.connect(self._on_open_langkit)
        self._keyboard_shortcut.setAutoRepeat(False)

    def _fix_deck_shortcut(self):
        """Fix the deck shortcut to work when Langkit is open."""
        # Store original moveToState method
        self._original_moveToState = mw.moveToState

        def moveToState_wrapper(state, *args):
            # If trying to go to deck browser and Langkit is visible, close it first
            if state == "deckBrowser" and hasattr(mw, '_langkit_visible') and mw._langkit_visible:
                if self.webview_tab:
                    self.webview_tab.hide()
            # Then proceed with the original state change
            return self._original_moveToState(state, *args)

        # Replace moveToState with our wrapper
        mw.moveToState = moveToState_wrapper

    def _setup_menu(self):
        """Add Langkit menu items."""
        menu = QMenu("Langkit", mw)
        mw.menuBar().addMenu(menu)
        
        # Open Langkit action
        open_action = QAction("Open Langkit", mw)
        qconnect(open_action.triggered, self._on_open_langkit)
        menu.addAction(open_action)
        
        menu.addSeparator()
        
        # Server control actions
        start_action = QAction("Start Server", mw)
        qconnect(start_action.triggered, self._start_server)
        menu.addAction(start_action)
        
        stop_action = QAction("Stop Server", mw)
        qconnect(stop_action.triggered, self._stop_server)
        menu.addAction(stop_action)
        
        restart_action = QAction("Restart Server", mw)
        qconnect(restart_action.triggered, self._restart_server)
        menu.addAction(restart_action)
        
        menu.addSeparator()
        
        # Download action
        download_action = QAction("Download Langkit Application", mw)
        qconnect(download_action.triggered, self._download_binary_manual)
        menu.addAction(download_action)
        
        # Update action
        update_action = QAction("Check for Updates", mw)
        qconnect(update_action.triggered, self._check_for_updates_manual)
        menu.addAction(update_action)
        
        # Diagnostics action
        diag_action = QAction("Show Diagnostics", mw)
        qconnect(diag_action.triggered, self._show_diagnostics)
        menu.addAction(diag_action)
        
    def _on_open_langkit(self):
        """Open Langkit interface."""
        # Check if we need to download the binary first
        if not self.binary_manager.check_binary_exists():
            binary_path = self.binary_manager.download_with_confirmation()
            if not binary_path:
                # User cancelled download or download failed
                return
            
            self._save_config()
            
            # Binary downloaded successfully, check OS compatibility
            self._check_and_prompt_os_compatibility()
            
            # Create process manager
            self.process_manager = ProcessManager(binary_path, self.config)
            
            # Create webview tab
            self.webview_tab = LangkitTab(self.process_manager)
        else:
            # Binary exists - check for updates first
            if self.config.get("auto_update", True) and not self.update_checked_this_session:
                self.update_checked_this_session = True
                new_version = self.binary_manager.check_for_updates()
                if new_version:
                    # Show update notification
                    current = self.config.get("last_known_version", "unknown")
                    msg = f"A new version of Langkit is available!\n\n"
                    msg += f"Current version: {current}\n"
                    msg += f"New version: {new_version}\n\n"
                    msg += "Would you like to update now?"
                    
                    ret = QMessageBox.question(
                        mw,
                        "Langkit Update Available",
                        msg,
                        QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No
                    )
                    
                    if ret == QMessageBox.StandardButton.Yes:
                        # Perform update
                        if self.binary_manager.update_binary():
                            # Save the updated config (including new version)
                            self._save_config()
                            showInfo("Langkit updated successfully! Please click Langkit again to start.")
                            return  # Exit without starting the old binary
                        else:
                            showWarning("Failed to update Langkit")
                            # Continue with old version
            
            # Check OS compatibility if not already checked
            self._check_and_prompt_os_compatibility()
            
            # Ensure process manager and webview are created
            if not self.process_manager:
                binary_path = self.binary_manager.get_binary_path_if_exists()
                if binary_path:
                    self.process_manager = ProcessManager(binary_path, self.config)
                    
            if not self.webview_tab and self.process_manager:
                self.webview_tab = LangkitTab(self.process_manager)
                
        # Show the interface
        if self.webview_tab:
            # show() will handle server startup and clean up on failure
            self.webview_tab.show()
            
    def _start_server(self):
        """Start the Langkit server."""
        # Ensure binary exists
        if not self.binary_manager.check_binary_exists():
            showWarning("Langkit application is not installed. Click the Langkit button to download it.")
            return
            
        # Create process manager if needed
        if not self.process_manager:
            binary_path = self.binary_manager.get_binary_path_if_exists()
            if binary_path:
                self.process_manager = ProcessManager(binary_path, self.config)
            else:
                showWarning("Could not find Langkit binary")
                return
            
        if self.process_manager.is_running():
            showInfo("Langkit server is already running")
            return
            
        if self.process_manager.start():
            showInfo("Langkit server started successfully")
        else:
            showWarning("Failed to start Langkit server")
            
    def _stop_server(self):
        """Stop the Langkit server."""
        if not self.process_manager:
            return
            
        self.process_manager.stop()
        showInfo("Langkit server stopped")
        
    def _restart_server(self):
        """Restart the Langkit server."""
        if not self.process_manager:
            return
            
        if self.process_manager.restart():
            showInfo("Langkit server restarted successfully")
        else:
            showWarning("Failed to restart Langkit server")
            
    def _download_binary_manual(self):
        """Manually trigger binary download."""
        if self.binary_manager.check_binary_exists():
            showInfo("Langkit application is already installed.")
            return
            
        binary_path = self.binary_manager.download_with_confirmation()
        if binary_path:
            # Check OS compatibility after successful download
            self._check_and_prompt_os_compatibility()
            
            self._save_config()
            
            # Create process manager with new binary
            self.process_manager = ProcessManager(binary_path, self.config)
            showInfo("Langkit application downloaded successfully!")
        
    def _check_for_updates_manual(self):
        """Manual update check."""
        if not self.binary_manager:
            return
            
        new_version = self.binary_manager.check_for_updates()
        if new_version:
            self._show_update_notification(new_version)
        else:
            showInfo("Langkit is up to date!")
            
    def _show_update_notification(self, new_version: str):
        """Show update available notification."""
        current = self.config.get("last_known_version")
        if not current:
            current = "unknown"
        msg = f"A new version of Langkit is available!\n\n"
        msg += f"Current version: {current}\n"
        msg += f"New version: {new_version}\n\n"
        msg += "Would you like to update now?"
        
        ret = QMessageBox.question(
            mw,
            "Langkit Update Available",
            msg,
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No
        )
        
        if ret == QMessageBox.StandardButton.Yes:
            self._perform_update()
            
    def _perform_update(self):
        """Perform the update."""
        # Stop server if running
        was_running = False
        if self.process_manager and self.process_manager.is_running():
            was_running = True
            self.process_manager.stop()
            
        # Update binary
        if self.binary_manager.update_binary():
            # Update process manager with new binary
            binary_path = self.binary_manager.get_binary_path()
            if binary_path:
                self.process_manager.binary_path = binary_path
                
            # Save updated config
            self._save_config()
            
            # Restart if was running
            if was_running:
                self.process_manager.start()
                
            showInfo("Langkit updated successfully!")
        else:
            showWarning("Failed to update Langkit")
            
    def _show_diagnostics(self):
        """Show diagnostic information."""
        info = ["Langkit Addon Diagnostics\n"]
        info.append(f"Addon Path: {self.addon_path}")
        info.append(f"Config: {self.config}")
        
        if self.binary_manager:
            platform_info = self.binary_manager.get_platform_info()
            info.append(f"\nPlatform: {platform_info}")
            binary_name = self.binary_manager.get_binary_name()
            info.append(f"Expected Binary: {binary_name}")
            
        if self.process_manager:
            diag = self.process_manager.get_diagnostics()
            info.append(f"\nProcess Diagnostics:")
            for key, value in diag.items():
                info.append(f"  {key}: {value}")
                
        showInfo("\n".join(info))
        
    def _on_config_changed(self):
        """Handle configuration changes."""
        self.config = mw.addonManager.getConfig(__name__)
        self._save_config()
        
    def _save_config(self):
        """Save configuration."""
        mw.addonManager.writeConfig(__name__, self.config)
        
    def _check_and_prompt_os_compatibility(self):
        """Check OS compatibility and show appropriate warnings."""
        # Check macOS compatibility warning
        if is_mac:
            warning_count = self.config.get("macos_untested_warning_count", 0)
            
            # Only show once
            if warning_count >= 1:
                return True
                
            # Show macOS compatibility notice
            msg = QMessageBox(mw)
            msg.setIcon(QMessageBox.Icon.Information)
            msg.setWindowTitle("macOS Compatibility Notice")
            msg.setText("<b>Langkit on macOS - Experimental</b>")
            
            info_text = (
                "Langkit has not been tested on macOS as the developer lacks access to Apple hardware.\n"
                "While the software may work as intended, you are likely to encounter unexpected issues.\n\n"
                "If you experience any problems, please report them on Github so that the developer can attempt to address them.\n\n"
                "Thank you for your understanding."
            )
            
            msg.setInformativeText(info_text)
            msg.setStandardButtons(QMessageBox.StandardButton.Ok)
            msg.exec()
            
            # Mark as shown
            self.config["macos_untested_warning_count"] = 1
            self._save_config()
            
            return True
        
        # Windows Direct3D check
        if not is_win:
            return True
            
        current_driver = mw.pm.video_driver()
        
        # Only warn about Direct3D on high refresh rate displays
        if current_driver != VideoDriver.Direct3D:
            return True
            
        # Check warning count
        warning_count = self.config.get("direct3d_refresh_warning_count", 0)
        
        # Don't show after 2 warnings
        if warning_count >= 2:
            return True
            
        # Show warning about high refresh rate displays
        msg = QMessageBox(mw)
        msg.setIcon(QMessageBox.Icon.Information)
        msg.setWindowTitle("Display Configuration Notice")
        msg.setText("<b>Using Direct3D with High Refresh Rate Display</b>")
        
        # Different message for second warning (reminder)
        if warning_count == 1:
            info_text = (
                "Reminder: If you're experiencing visual glitches or flickering in Langkit, "
                "it may be due to your high refresh rate display.\n\n"
                "Try lowering your display refresh rate to 60Hz in Windows Display Settings "
                "or switch to OpenGL in Anki's preferences (though performance may be reduced).\n\n"
                "No further reminder will occur."
            )
        else:
            # First time warning
            info_text = (
                "Langkit works best with Direct3D for performance, but some users with "
                "high refresh rate displays (>60Hz) may experience visual glitches.\n\n"
                "If you notice any flickering or visual issues:\n"
                "• Try lowering your display refresh rate to 60Hz in Windows Display Settings\n"
                "• Or switch to Video Driver : OpenGL in Anki's preferences (though performance will suffer)\n\n"
                "Most users will not experience any issues."
            )
        
        msg.setInformativeText(info_text)
        msg.setStandardButtons(QMessageBox.StandardButton.Ok)
        msg.exec()
        
        # Increment warning count
        self.config["direct3d_refresh_warning_count"] = warning_count + 1
        self._save_config()
        
        return True
        
    def cleanup(self):
        """Clean up resources on shutdown."""
        # Remove toolbar hook callback to prevent duplicates
        if self._toolbar_hook_callback:
            gui_hooks.top_toolbar_did_init_links.remove(self._toolbar_hook_callback)
            self._toolbar_hook_callback = None

        # Clean up keyboard shortcut
        if self._keyboard_shortcut:
            self._keyboard_shortcut.deleteLater()
            self._keyboard_shortcut = None

        # Restore original moveToState method
        if self._original_moveToState:
            mw.moveToState = self._original_moveToState
            self._original_moveToState = None

        if self.webview_tab:
            self.webview_tab.cleanup()

        if self.process_manager:
            self.process_manager.stop()


# Global addon instance
addon = None


def initialize_addon():
    """Initialize the addon after profile load."""
    global addon
    addon = LangkitAddon()
    addon.initialize()


def cleanup_addon():
    """Clean up on profile close."""
    global addon
    if addon:
        addon.cleanup()
        addon = None


# Register hooks
gui_hooks.profile_did_open.append(initialize_addon)
gui_hooks.profile_will_close.append(cleanup_addon)