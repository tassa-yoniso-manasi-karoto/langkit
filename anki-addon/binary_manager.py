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
import subprocess
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


def parse_version(v_str: str) -> tuple:
    """
    Parse a version string like "v1.2.3" into a tuple of integers (1, 2, 3).
    Handles 'v' prefix and non-numeric parts gracefully.
    """
    # Remove 'v' prefix if it exists
    if v_str.startswith('v'):
        v_str = v_str[1:]
    
    parts = v_str.split('.')
    version_parts = []
    for part in parts:
        try:
            # Keep only the numeric part (e.g., "10-hotfix" -> 10)
            numeric_part = "".join(filter(str.isdigit, part))
            if numeric_part:
                version_parts.append(int(numeric_part))
        except (ValueError, TypeError):
            # If a part is not a valid number, skip it or default to 0
            version_parts.append(0)
            
    return tuple(version_parts)

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
        self.timeout = config.get("download_timeout", 600)
        # Session-only cache for Linux webkit binary detection
        self._working_binary_cache = None

        # Handle migration from old binary names to new ones
        self._migrate_old_binaries()

    def _migrate_old_binaries(self):
        """Migrate old binary names to new format for seamless updates."""
        if platform.system() == "Linux":
            # Migrate from langkit-app-linux to langkit
            old_binary = self.binaries_dir / "langkit-app-linux"
            new_binary = self.binaries_dir / "langkit"

            if old_binary.exists() and not new_binary.exists():
                try:
                    print(f"[BinaryManager] Migrating old binary from 'langkit-app-linux' to 'langkit'")
                    shutil.move(str(old_binary), str(new_binary))
                    # Ensure executable permissions
                    st = os.stat(new_binary)
                    os.chmod(new_binary, st.st_mode | stat.S_IEXEC)
                    print(f"[BinaryManager] Migration successful")
                except Exception as e:
                    print(f"[BinaryManager] Migration failed: {e}")

        elif platform.system() == "Windows":
            # For consistency, also handle potential Windows renames if needed in future
            # Currently Windows binary naming hasn't changed (still langkit-app.exe)
            pass

        elif platform.system() == "Darwin":
            # macOS naming also hasn't changed (still langkit.app)
            pass

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

    def _detect_working_binary_linux(self) -> Optional[str]:
        """Detect which webkit binary works on Linux. Cache valid only for this session."""
        if platform.system() != "Linux":
            return None

        # Use cached result if available (session-only)
        if self._working_binary_cache is not None:
            return self._working_binary_cache

        # Try binaries in order of preference (webkit2gtk-4.0 is more common)
        for binary_name in ["langkit", "langkit-webkit2_41"]:
            binary_path = self.binaries_dir / binary_name
            if binary_path.exists():
                try:
                    # Quick test with timeout to see if binary runs
                    result = subprocess.run(
                        [str(binary_path), "--version"],
                        capture_output=True,
                        timeout=2,
                        text=True
                    )
                    if result.returncode == 0:
                        print(f"[BinaryManager] Detected working Linux binary: {binary_name}")
                        self._working_binary_cache = binary_name
                        return binary_name
                except (subprocess.TimeoutExpired, OSError) as e:
                    print(f"[BinaryManager] Binary {binary_name} failed to run: {e}")
                    continue

        # Neither binary worked
        print("[BinaryManager] Warning: No working Linux binary found")
        return None
    
    def get_local_binary_name(self, compressed_name: Optional[str] = None) -> Optional[str]:
        """Get the local binary name after extraction from compressed file."""
        if compressed_name is None:
            compressed_name = self.get_binary_name()

        if not compressed_name:
            return None

        # Platform-specific naming
        if platform.system() == "Darwin":
            return "langkit.app"
        elif platform.system() == "Windows":
            # Windows always uses langkit-app.exe regardless of archive name
            return "langkit-app.exe"
        else:
            # Linux - need to detect which webkit version works
            return self._detect_working_binary_linux()
    
    def check_binary_exists(self) -> bool:
        """Check if the langkit binary exists without downloading."""
        # Check if binary path is configured
        if self.config.get("binary_path"):
            path = Path(self.config["binary_path"])
            return path.exists()

        if platform.system() == "Linux":
            # For Linux, check if at least one of the webkit variants exists
            for binary_variant in ["langkit", "langkit-webkit2_41"]:
                if (self.binaries_dir / binary_variant).exists():
                    return True
            return False
        else:
            # Check for existing binary on non-Linux platforms
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

        if platform.system() == "Linux":
            # For Linux, detect which webkit variant works
            working_binary = self._detect_working_binary_linux()
            if working_binary:
                binary_path = self.binaries_dir / working_binary
                if binary_path.exists():
                    # Ensure execute permissions
                    st = os.stat(binary_path)
                    os.chmod(binary_path, st.st_mode | stat.S_IEXEC)
                    return binary_path
            return None
        else:
            # Check for existing binary on non-Linux platforms
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
        msg = "The Langkit addon needs to download the Langkit application to run on your computer.\n\n"
        msg += "This is a one-time setup. Download and install now?"
        
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
                
            latest_version = release_info.get("tag_name", "0.0.0")
            current_version = self.config.get("last_known_version")
            
            if not current_version:
                return latest_version
                
            if parse_version(latest_version) > parse_version(current_version):
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
            
            with urllib.request.urlopen(req, timeout=5) as response:
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

                # Linux - ensure executable permissions for all binaries
                # The archive may contain multiple binaries (langkit and langkit-webkit2_41)
                for binary_variant in ["langkit", "langkit-webkit2_41"]:
                    variant_path = self.binaries_dir / binary_variant
                    if variant_path.exists():
                        st = os.stat(variant_path)
                        os.chmod(variant_path, st.st_mode | stat.S_IEXEC)

                # Try to detect which binary actually works
                working_binary = self._detect_working_binary_linux()
                if working_binary:
                    final_path = self.binaries_dir / working_binary
                else:
                    # Fallback to first existing binary
                    for binary_variant in ["langkit", "langkit-webkit2_41"]:
                        variant_path = self.binaries_dir / binary_variant
                        if variant_path.exists():
                            final_path = variant_path
                            break
                    
            else:
                # This shouldn't happen with current platform mappings
                # but handle it just in case
                showCritical(f"Unknown file format: {binary_name}", title="Download failed")
                temp_path.unlink()
                return None
                    
            # Update config with version
            self.config["last_known_version"] = release_info.get("tag_name", "").lstrip("v")
            
            progress.close()
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

        # Clear the webkit binary cache on Linux for the update
        if platform.system() == "Linux":
            self._working_binary_cache = None

        # Backup current binaries
        backed_up_files = []
        if platform.system() == "Linux":
            # On Linux, backup both webkit variants if they exist
            for binary_variant in ["langkit", "langkit-webkit2_41"]:
                current_path = self.binaries_dir / binary_variant
                if current_path.exists():
                    backup_path = self.binaries_dir / f"{binary_variant}.backup"
                    if backup_path.exists():
                        backup_path.unlink()
                    shutil.copy2(current_path, backup_path)
                    backed_up_files.append((current_path, backup_path))
                    # Remove current binary
                    current_path.unlink()
        else:
            # Non-Linux platforms
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
                    backed_up_files.append((current_path, backup_path))

                    # Remove current binary
                    if current_path.is_dir():
                        shutil.rmtree(current_path)
                    else:
                        current_path.unlink()
                    
        # Download new version
        new_binary = self._download_binary()
        if new_binary:
            # Remove backups on success
            for _, backup_path in backed_up_files:
                if backup_path.exists():
                    if backup_path.is_dir():
                        shutil.rmtree(backup_path)
                    else:
                        backup_path.unlink()
            return True
        else:
            # Restore backups on failure
            for original_path, backup_path in backed_up_files:
                if backup_path.exists():
                    if backup_path.is_dir():
                        shutil.copytree(backup_path, original_path)
                    else:
                        shutil.copy2(backup_path, original_path)
                    # Clean up backup after restoration
                    if backup_path.is_dir():
                        shutil.rmtree(backup_path)
                    else:
                        backup_path.unlink()
            return False