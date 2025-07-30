"""
Binary management for Langkit addon.
Handles downloading, verification, and updates of the langkit executable.
"""

import hashlib
import json
import os
import platform
import shutil
import stat
import tarfile
import tempfile
import urllib.request
import urllib.error
import zipfile
from pathlib import Path
from typing import Dict, Optional, Tuple
from urllib.parse import urlparse

import aqt
from aqt.qt import *
from aqt.utils import showInfo, showWarning, showCritical


class BinaryManager:
    """Manages langkit binary lifecycle: download, verification, and updates."""
    
    GITHUB_API = "https://api.github.com/repos/{repo}/releases/latest"
    PLATFORM_MAPPING = {
        ("Windows", "AMD64"): "langkit-app-windows.zip",
        ("Darwin", "x86_64"): "langkit-app-macos.zip",
        ("Darwin", "arm64"): "langkit-app-macos.zip",  # Universal binary
        ("Linux", "x86_64"): "langkit-app-linux.tar.xz",
    }
    
    def __init__(self, addon_path: Path, config: dict):
        self.addon_path = addon_path
        self.config = config
        self.binaries_dir = addon_path / "user_files" / "binaries"
        self.binaries_dir.mkdir(parents=True, exist_ok=True)
        self.github_repo = config.get("github_repo", "tassa-yoniso-manasi-karoto/langkit")
        self.timeout = config.get("download_timeout", 300)
        
    def get_platform_info(self) -> Tuple[str, str]:
        """Get current platform information."""
        system = platform.system()
        machine = platform.machine()
        
        # Normalize machine architecture
        if machine in ["x86_64", "AMD64", "amd64"]:
            machine = "AMD64" if system == "Windows" else "x86_64"
        elif machine in ["aarch64", "arm64"]:
            machine = "arm64"
            
        return system, machine
    
    def get_binary_name(self) -> Optional[str]:
        """Get the appropriate binary name for current platform."""
        platform_key = self.get_platform_info()
        return self.PLATFORM_MAPPING.get(platform_key)
    
    def get_local_binary_name(self, compressed_name: Optional[str] = None) -> Optional[str]:
        """Get the local binary name after extraction from compressed file."""
        if compressed_name is None:
            compressed_name = self.get_binary_name()
        
        if not compressed_name:
            return None
            
        # Remove compression extensions
        local_name = compressed_name.replace(".zip", "").replace(".tar.xz", "")
        
        # Platform-specific naming
        if platform.system() == "Darwin":
            return "langkit.app"
        elif platform.system() == "Windows":
            # Ensure Windows executables have .exe extension
            if not local_name.endswith(".exe"):
                return local_name + ".exe"
            return local_name
        else:
            # Linux keeps the name as-is
            return local_name
    
    def check_binary_exists(self) -> bool:
        """Check if the langkit binary exists without downloading."""
        # Check if binary path is configured
        if self.config.get("binary_path"):
            path = Path(self.config["binary_path"])
            return path.exists()
                
        # Check for existing binary
        local_name = self.get_local_binary_name()
        if not local_name:
            return False
            
        binary_path = self.binaries_dir / local_name
        return binary_path.exists()
    
    def get_binary_path_if_exists(self) -> Optional[Path]:
        """Get the path to the langkit binary only if it already exists."""
        # Check if binary path is configured
        if self.config.get("binary_path"):
            path = Path(self.config["binary_path"])
            if path.exists():
                # Ensure execute permissions on Unix systems
                if platform.system() != "Windows":
                    st = os.stat(path)
                    os.chmod(path, st.st_mode | stat.S_IEXEC)
                return path
                
        # Check for existing binary
        local_name = self.get_local_binary_name()
        if not local_name:
            return None
            
        binary_path = self.binaries_dir / local_name
        
        if binary_path.exists():
            # Ensure execute permissions on Unix systems
            if platform.system() != "Windows":
                st = os.stat(binary_path)
                os.chmod(binary_path, st.st_mode | stat.S_IEXEC)
            return binary_path
            
        return None
    
    def get_binary_path(self) -> Optional[Path]:
        """Get the path to the langkit binary, downloading if necessary."""
        # Try to get existing binary first
        existing_path = self.get_binary_path_if_exists()
        if existing_path:
            return existing_path
            
        # Check platform support
        binary_name = self.get_binary_name()
        if not binary_name:
            showCritical(
                f"Langkit is not available for {platform.system()} {platform.machine()}",
                title="Unsupported platform"
            )
            return None
            
        # Download binary
        return self._download_binary()
    
    def download_with_confirmation(self) -> Optional[Path]:
        """Download the langkit binary after showing user-friendly confirmation dialog."""
        # Check platform support first
        binary_name = self.get_binary_name()
        if not binary_name:
            showCritical(
                f"Langkit is not available for {platform.system()} {platform.machine()}",
                title="Unsupported platform"
            )
            return None
            
        # Show user-friendly confirmation dialog
        msg = "The Langkit addon needs to download the Langkit application "
        msg += "(approximately 95MB) to run on your computer.\n\n"
        msg += "This is a one-time setup that installs the language learning tools.\n\n"
        msg += "Download and install now?"
        
        ret = QMessageBox.question(
            aqt.mw,
            "Setup Required",
            msg,
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.Yes
        )
        
        if ret == QMessageBox.StandardButton.Yes:
            return self._download_binary()
        else:
            return None
    
    def check_for_updates(self) -> Optional[str]:
        """Check if a newer version is available on GitHub."""
        try:
            # Get latest release info
            release_info = self._fetch_release_info()
            if not release_info:
                return None
                
            latest_version = (release_info.get("tag_name") or "").lstrip("v")
            current_version = (self.config.get("last_known_version") or "").lstrip("v")
            
            if not current_version:
                return latest_version
                
            # Simple version comparison (could use semver for more robust comparison)
            if latest_version > current_version:
                return latest_version
                
        except Exception as e:
            print(f"Error checking for updates: {e}")
            
        return None
    
    def _fetch_release_info(self) -> Optional[Dict]:
        """Fetch latest release information from GitHub API."""
        try:
            url = self.GITHUB_API.format(repo=self.github_repo)
            req = urllib.request.Request(url, headers={
                "Accept": "application/vnd.github.v3+json",
                "User-Agent": "Langkit-Anki-Addon"
            })
            
            with urllib.request.urlopen(req, timeout=10) as response:
                return json.loads(response.read().decode())
                
        except Exception as e:
            print(f"Failed to fetch release info: {e}")
            return None
    
    def _download_binary(self) -> Optional[Path]:
        """Download the langkit binary with progress dialog."""
        release_info = self._fetch_release_info()
        if not release_info:
            showCritical("Could not fetch release information from GitHub", title="Download failed")
            return None
            
        binary_name = self.get_binary_name()
        if not binary_name:
            return None
            
        # Find the asset
        asset = None
        for a in release_info.get("assets", []):
            if a["name"] == binary_name:
                asset = a
                break
                
        if not asset:
            showCritical(f"Could not find {binary_name} in latest release", title="Download failed")
            return None
            
        # Create progress dialog
        progress = QProgressDialog("Downloading Langkit...", "Cancel", 0, 100, aqt.mw)
        progress.setWindowModality(Qt.WindowModality.WindowModal)
        progress.setAutoClose(True)
        progress.setMinimumDuration(0)
        
        try:
            # Download to temporary file
            temp_path = self.binaries_dir / f"{binary_name}.tmp"
            download_url = asset["browser_download_url"]
            expected_hash = asset.get("digest", "").replace("sha256:", "")
            
            def download_hook(block_num, block_size, total_size):
                if progress.wasCanceled():
                    raise Exception("Download cancelled")
                if total_size > 0:
                    downloaded = block_num * block_size
                    percent = min(int(downloaded * 100 / total_size), 100)
                    progress.setValue(percent)
                    QApplication.processEvents()
                    
            urllib.request.urlretrieve(download_url, temp_path, reporthook=download_hook)
            
            # Verify checksum if available
            if expected_hash:
                progress.setLabelText("Verifying download...")
                if not self._verify_checksum(temp_path, expected_hash):
                    temp_path.unlink()
                    showCritical(
                        "Downloaded file checksum does not match. Please try again.",
                        title="Verification failed"
                    )
                    return None
                    
            # Extract compressed files
            final_path = None
            local_name = self.get_local_binary_name(binary_name)
            
            if binary_name.endswith(".zip"):
                progress.setLabelText("Extracting...")
                with zipfile.ZipFile(temp_path, 'r') as zip_ref:
                    zip_ref.extractall(self.binaries_dir)
                temp_path.unlink()
                
                # Find the extracted file
                if platform.system() == "Darwin":
                    # Find the app bundle on macOS
                    for item in self.binaries_dir.iterdir():
                        if item.name.endswith(".app"):
                            final_path = item
                            break
                elif platform.system() == "Windows":
                    # Windows executable should be directly in the zip
                    final_path = self.binaries_dir / local_name
                    
            elif binary_name.endswith(".tar.xz"):
                progress.setLabelText("Extracting...")
                with tarfile.open(temp_path, 'r:xz') as tar_ref:
                    tar_ref.extractall(self.binaries_dir)
                temp_path.unlink()
                
                # Linux binary should be extracted with its original name
                final_path = self.binaries_dir / local_name
                
                # Ensure executable permissions
                if final_path.exists():
                    st = os.stat(final_path)
                    os.chmod(final_path, st.st_mode | stat.S_IEXEC)
                    
            else:
                # This shouldn't happen with current platform mappings
                # but handle it just in case
                showCritical(f"Unknown file format: {binary_name}", title="Download failed")
                temp_path.unlink()
                return None
                    
            # Update config with version
            self.config["last_known_version"] = release_info.get("tag_name", "").lstrip("v")
            
            progress.close()
            showInfo(f"Langkit downloaded successfully to {final_path}")
            return final_path
            
        except Exception as e:
            progress.close()
            if temp_path.exists():
                temp_path.unlink()
            showCritical(str(e), title="Download failed")
            return None
    
    def _verify_checksum(self, file_path: Path, expected_hash: str) -> bool:
        """Verify SHA256 checksum of downloaded file."""
        sha256 = hashlib.sha256()
        with open(file_path, 'rb') as f:
            while True:
                data = f.read(65536)  # Read in 64KB chunks
                if not data:
                    break
                sha256.update(data)
                
        actual_hash = sha256.hexdigest()
        return actual_hash.lower() == expected_hash.lower()
    
    def update_binary(self) -> bool:
        """Update the binary to latest version."""
        # First check if update is available
        new_version = self.check_for_updates()
        if not new_version:
            showInfo("Langkit is already up to date.", title="No updates available")
            return False
            
        # Backup current binary
        local_name = self.get_local_binary_name()
        if local_name:
            current_path = self.binaries_dir / local_name
            if current_path.exists():
                backup_path = self.binaries_dir / f"{local_name}.backup"
                if backup_path.exists():
                    if backup_path.is_dir():
                        shutil.rmtree(backup_path)
                    else:
                        backup_path.unlink()
                        
                if current_path.is_dir():
                    shutil.copytree(current_path, backup_path)
                else:
                    shutil.copy2(current_path, backup_path)
                    
                # Remove current binary
                if current_path.is_dir():
                    shutil.rmtree(current_path)
                else:
                    current_path.unlink()
                    
        # Download new version
        new_binary = self._download_binary()
        if new_binary:
            # Remove backup on success
            if 'backup_path' in locals() and backup_path.exists():
                if backup_path.is_dir():
                    shutil.rmtree(backup_path)
                else:
                    backup_path.unlink()
            return True
        else:
            # Restore backup on failure
            if 'backup_path' in locals() and backup_path.exists():
                if backup_path.is_dir():
                    shutil.copytree(backup_path, current_path)
                else:
                    shutil.copy2(backup_path, current_path)
            return False