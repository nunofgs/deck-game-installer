import subprocess
import sys
from pathlib import Path
from typing import List, Optional, Tuple


def _pyside_available() -> bool:
    try:
        import PySide6  # noqa: F401
        return True
    except Exception:
        return False

class KDialog:
    @staticmethod
    def _run(args: List[str]) -> subprocess.CompletedProcess:
        try:
            return subprocess.run(["kdialog"] + args, capture_output=True, text=True)
        except FileNotFoundError:
            # Last resort: write to stderr
            sys.stderr.write("kdialog is not available.\n")
            return subprocess.CompletedProcess(args, returncode=1, stdout="", stderr="kdialog not found")

    @classmethod
    def error(cls, title: str, message: str):
        cls._run(["--title", title, "--error", message])

    @classmethod
    def info(cls, title: str, message: str):
        cls._run(["--title", title, "--msgbox", message])

    @classmethod
    def question(cls, title: str, message: str) -> bool:
        result = cls._run(["--title", title, "--yesno", message])
        return result.returncode == 0

    @classmethod
    def select_file(cls, title: str, start_dir: Path, filter: str) -> Optional[Path]:
        result = cls._run(["--title", title, "--getopenfilename", str(start_dir), filter])
        if result.returncode == 0:
            return Path(result.stdout.strip())
        return None

    @classmethod
    def radio_list(cls, title: str, options: List[Tuple[str, str]], message: str = "Select an option:", default: str = None) -> Optional[str]:
        # options: [(tag, display_text), ...]
        # kdialog --radiolist <text> [tag item status]...
        args = ["--title", title, "--radiolist", message]
        for tag, display_text in options:
            status = "on" if tag == default else "off"
            args.extend([tag, display_text, status])
        
        result = cls._run(args)
        if result.returncode == 0:
            return result.stdout.strip()
        return None

    @classmethod
    def combo_box(cls, title: str, message: str, options: List[str], default: str = None) -> Optional[str]:
        # kdialog --combobox <text> [item]...
        args = ["--title", title, "--combobox", message]
        args.extend(options)
        # Note: kdialog combo box doesn't have a built-in "default" selection in the same way radiolist does
        # it just shows the items in order.
        result = cls._run(args)
        if result.returncode == 0:
            return result.stdout.strip()
        return None
    @classmethod
    def text_box(cls, title: str, text: str, width: int = 600, height: int = 400):
        # Using temporary file for textbox content
        import tempfile
        import os
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as tf:
            tf.write(text)
            temp_name = tf.name
        
        try:
            cls._run(["--title", title, "--textbox", temp_name, str(width), str(height)])
        finally:
            if os.path.exists(temp_name):
                os.remove(temp_name)

class Logger:
    def __init__(self):
        self.logs = []
        self.on_log = None

    def log(self, message: str):
        print(f"[LOG] {message}")
        self.logs.append(message)
        if self.on_log:
            self.on_log(message)

    def get_text(self) -> str:
        return "\n".join(self.logs)

    def get_recent(self, n: int = 10) -> str:
        return "\n".join(self.logs[-n:])

class LogWindow:
    def __init__(self, title: str):
        self.title = title
        self.process = None
        self._current_content = ""
        self._use_internal = _pyside_available()

    def open(self, ok_label: str = "OK", cancel_label: str = "Cancel"):
        """Opens the log window. ok_label/cancel_label are for Zenity buttons."""
        if not self._use_internal:
            raise RuntimeError(
                "PySide6 is not available for the log window. "
                "Install it with 'pip install PySide6' and try again."
            )

        cmd = [
            sys.executable,
            "-m",
            "deck_game_installer.log_window",
            "--title",
            self.title,
            "--ok-label",
            ok_label,
            "--cancel-label",
            cancel_label,
        ]

        self.process = subprocess.Popen(cmd, stdin=subprocess.PIPE, text=True)
        # If we have existing content, write it in
        if self._current_content:
            try:
                self.process.stdin.write(self._current_content)
                self.process.stdin.flush()
            except BrokenPipeError:
                pass

    def write(self, message: str):
        msg_with_newline = message + "\n"
        self._current_content += msg_with_newline
        if self.process and self.process.poll() is None:
            try:
                self.process.stdin.write(msg_with_newline)
                self.process.stdin.flush()
            except BrokenPipeError:
                pass

    def close(self):
        if self.process:
            self.process.terminate()
            self.process = None

    def wait(self) -> bool:
        """Waits for the user to close the window (e.g. by clicking OK). Returns True if OK was clicked."""
        if self.process:
            self.process.stdin.close() # Signal EOF so Zenity enables OK button if it was waiting
            self.process.wait()
            return self.process.returncode == 0
        return False
