"""
Process management for Langkit server.
Handles subprocess lifecycle, port discovery, and health monitoring.
"""

import atexit
import json
import os
import platform
import signal
import subprocess
import tempfile
import threading
import time
from pathlib import Path
from typing import Dict, Optional, Tuple

from aqt.utils import showWarning, showCritical


class ProcessManager:
    """Manages the langkit server subprocess."""
    
    def __init__(self, binary_path: Path, config: dict):
        self.binary_path = binary_path
        self.config = config
        self.process: Optional[subprocess.Popen] = None
        self.server_config: Optional[Dict] = None
        self.config_file: Optional[Path] = None
        self.monitor_thread: Optional[threading.Thread] = None
        self.shutdown_event = threading.Event()
        self.startup_timeout = config.get("process_startup_timeout", 30)
        self.poll_interval = config.get("config_poll_interval", 0.5)
        self.poll_timeout = config.get("config_poll_timeout", 10)
        self.last_stderr = ""  # Store stderr from monitor thread
        
        # Register cleanup on exit
        atexit.register(self.stop)
        
    def start(self) -> bool:
        """Start the langkit server process."""
        if self.is_running():
            return True
            
        # Clear any previous stderr
        self.last_stderr = ""
            
        try:
            # Create temporary config file
            fd, config_path = tempfile.mkstemp(suffix=".json", prefix="langkit_addon_")
            os.close(fd)  # Close the file descriptor
            self.config_file = Path(config_path)
            
            # Write initial config
            initial_config = {
                "addon_instance": True,
                "created_at": time.time()
            }
            with open(self.config_file, 'w') as f:
                json.dump(initial_config, f, indent=2)
                
            # Prepare command
            if platform.system() == "Darwin" and self.binary_path.suffix == ".app":
                # macOS app bundle
                cmd = [str(self.binary_path / "Contents" / "MacOS" / "langkit")]
            else:
                cmd = [str(self.binary_path)]
                
            cmd.extend(["--server", str(self.config_file)])
            
            # Start process
            # Use CREATE_NO_WINDOW on Windows to prevent console window
            startupinfo = None
            if platform.system() == "Windows":
                startupinfo = subprocess.STARTUPINFO()
                startupinfo.dwFlags |= subprocess.STARTF_USESHOWWINDOW
                
            self.process = subprocess.Popen(
                cmd,
                stdout=None, # CRITICAL: Do NOT monitor with PIPE
                stderr=None, # see https://github.com/ankitects/anki/issues/4230#issuecomment-3127202125
                startupinfo=startupinfo,
                text=True,
                bufsize=1  # Line buffered
            )
            
            # Start monitor thread
            self.shutdown_event.clear()
            self.monitor_thread = threading.Thread(target=self._monitor_process, daemon=True)
            self.monitor_thread.start()
            
            # Wait for server to write port information
            if not self._wait_for_ports():
                self.stop()
                return False
                
            return True
            
        except Exception as e:
            showCritical(str(e), title="Failed to start Langkit")
            self.stop()
            return False
            
    def stop(self):
        """Stop the langkit server process gracefully."""
        self.shutdown_event.set()
        
        if self.process:
            try:
                # Try graceful shutdown first
                if platform.system() == "Windows":
                    self.process.terminate()
                else:
                    self.process.send_signal(signal.SIGTERM)
                    
                # Wait up to 5 seconds for graceful shutdown
                try:
                    self.process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    # Force kill if needed
                    self.process.kill()
                    self.process.wait()
                    
            except Exception as e:
                print(f"Error stopping process: {e}")
                
            self.process = None
            
        # Clean up config file
        if self.config_file and self.config_file.exists():
            try:
                self.config_file.unlink()
            except Exception:
                pass
                
        self.server_config = None
        
    def restart(self) -> bool:
        """Restart the langkit server."""
        self.stop()
        time.sleep(1)  # Brief pause before restart
        return self.start()
        
    def is_running(self) -> bool:
        """Check if the server process is running."""
        return self.process is not None and self.process.poll() is None
        
    def get_server_info(self) -> Optional[Dict]:
        """Get server configuration including ports."""
        return self.server_config
        
    def get_frontend_url(self) -> Optional[str]:
        """Get the frontend URL if server is running."""
        if self.server_config and "langkit_server" in self.server_config:
            server_info = self.server_config["langkit_server"]
            # Check for single-port mode first
            if server_info.get("single_port"):
                port = server_info.get("port")
            else:
                # Fallback to frontend_port for backward compatibility
                port = server_info.get("frontend_port")
            if port:
                return f"http://localhost:{port}"
        return None
        
    def _wait_for_ports(self) -> bool:
        """Wait for langkit to write port information to config file."""
        start_time = time.time()
        
        while time.time() - start_time < self.poll_timeout:
            if not self.is_running():
                # Process died during startup
                exit_code = self.process.returncode if self.process else None
                
                # First check if monitor thread already captured stderr
                stderr = self.last_stderr
                
                # If not, try to get it ourselves
                if not stderr and self.process:
                    # For processes that die quickly, use communicate to get all output
                    try:
                        # communicate() will wait for process to finish and return all output
                        stdout, stderr_output = self.process.communicate(timeout=1.0)
                        stderr = stderr_output
                    except subprocess.TimeoutExpired:
                        # Process still running somehow, kill it
                        self.process.kill()
                        stdout, stderr_output = self.process.communicate()
                        stderr = stderr_output
                    except:
                        # Fallback to reading what we can
                        stderr = self._read_stderr()
                
                # Ensure we have exit code after communicate
                if self.process:
                    exit_code = self.process.returncode
                
                # Check for dynamic linking errors
                if exit_code == 127 or self._is_linking_error(stderr):
                    error_msg = "Langkit cannot run in this environment due to missing system libraries.\n\n"
                    error_msg += "This typically happens when:\n"
                    error_msg += "• Running Anki from Flatpak or Snap\n"
                    error_msg += "• Missing WebView libraries\n\n"
                    error_msg += "Please install Anki from https://apps.ankiweb.net/ instead.\n\n"
                    error_msg += f"Technical details:\n{stderr}"
                    
                    showCritical(error_msg, title="Incompatible Environment")
                else:
                    showCritical(
                        f"Process terminated unexpectedly.\n\n{stderr}",
                        title="Langkit failed to start"
                    )
                return False
                
            try:
                # Read config file
                with open(self.config_file, 'r') as f:
                    config = json.load(f)
                    
                # Check if langkit has written server info
                if "langkit_server" in config:
                    server_info = config["langkit_server"]
                    required_keys = ["frontend_port", "api_port", "ws_port"]
                    
                    if all(key in server_info for key in required_keys):
                        self.server_config = config
                        print(f"Langkit server started: {server_info}")
                        return True
                        
            except (json.JSONDecodeError, FileNotFoundError):
                # File not ready yet or invalid JSON
                pass
                
            time.sleep(self.poll_interval)
            
        # Timeout
        stderr = self._read_stderr()
        showCritical(
            f"Server did not provide port information within {self.poll_timeout} seconds.\n\n{stderr}",
            title="Langkit startup timeout"
        )
        return False
        
    def _monitor_process(self):
        """Monitor the subprocess for crashes and collect output."""
        while not self.shutdown_event.is_set():
            if self.process and self.process.poll() is not None:
                # Process has terminated
                if not self.shutdown_event.is_set():
                    # Unexpected termination
                    returncode = self.process.returncode
                    stderr = self._read_stderr()
                    
                    # Store stderr for other methods to access
                    self.last_stderr = stderr
                    
                    print(f"Langkit process terminated unexpectedly with code {returncode}")
                    if stderr:
                        print(f"Error output: {stderr}")
                        
                    # Could implement auto-restart logic here if desired
                    self.process = None
                    self.server_config = None
                    
                break
                
            time.sleep(1)
            
    def _read_stderr(self) -> str:
        """Read available stderr output from process."""
        if not self.process or not self.process.stderr:
            return ""
            
        try:
            # Non-blocking read of available data
            import select
            stderr_lines = []
            
            if platform.system() != "Windows":
                # Unix: use select for non-blocking read
                while True:
                    ready, _, _ = select.select([self.process.stderr], [], [], 0)
                    if ready:
                        line = self.process.stderr.readline()
                        if line:
                            stderr_lines.append(line.strip())
                        else:
                            break
                    else:
                        break
            else:
                # Windows: read with timeout (less elegant but works)
                import queue
                import threading
                
                q = queue.Queue()
                
                def reader():
                    try:
                        for line in self.process.stderr:
                            q.put(line.strip())
                    except:
                        pass
                        
                reader_thread = threading.Thread(target=reader)
                reader_thread.daemon = True
                reader_thread.start()
                
                # Collect lines for up to 0.1 seconds
                deadline = time.time() + 0.1
                while time.time() < deadline:
                    try:
                        line = q.get(timeout=0.01)
                        stderr_lines.append(line)
                    except queue.Empty:
                        break
                        
            return "\n".join(stderr_lines[-20:])  # Last 20 lines
            
        except Exception as e:
            return f"Error reading stderr: {e}"
            
    def _is_linking_error(self, stderr: str) -> bool:
        """Check if the error is related to missing dynamic libraries."""
        if not stderr:
            return False
            
        linking_patterns = [
            # Linux
            "error while loading shared libraries",
            "cannot open shared object file",
            # Windows
            "The code execution cannot proceed because",
            "was not found",
            "is missing from your computer",
            # macOS
            "dyld: Library not loaded",
            "dyld: Symbol not found",
            "Reason: image not found",
        ]
        
        stderr_lower = stderr.lower()
        return any(pattern.lower() in stderr_lower for pattern in linking_patterns)
    
    def get_diagnostics(self) -> Dict:
        """Get diagnostic information about the process."""
        return {
            "running": self.is_running(),
            "pid": self.process.pid if self.process else None,
            "binary_path": str(self.binary_path),
            "config_file": str(self.config_file) if self.config_file else None,
            "server_config": self.server_config,
            "frontend_url": self.get_frontend_url()
        }