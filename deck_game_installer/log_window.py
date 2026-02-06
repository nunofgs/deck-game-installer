import argparse
import queue
import sys
import threading
from PySide6 import QtCore, QtWidgets


class _LogWindow(QtWidgets.QWidget):
    def __init__(self, title: str, ok_label: str, cancel_label: str):
        super().__init__()
        self._exit_code = 1
        self._queue: queue.Queue[str | None] = queue.Queue()
        self._stop_event = threading.Event()

        self.setWindowTitle(title)
        self.resize(700, 500)

        layout = QtWidgets.QVBoxLayout(self)

        self.text = QtWidgets.QPlainTextEdit(self)
        self.text.setReadOnly(True)
        layout.addWidget(self.text)

        button_layout = QtWidgets.QHBoxLayout()
        button_layout.addStretch(1)

        ok_btn = QtWidgets.QPushButton(ok_label, self)
        ok_btn.clicked.connect(self._on_ok)
        button_layout.addWidget(ok_btn)

        cancel_btn = QtWidgets.QPushButton(cancel_label, self)
        cancel_btn.clicked.connect(self._on_cancel)
        button_layout.addWidget(cancel_btn)

        layout.addLayout(button_layout)

        self._timer = QtCore.QTimer(self)
        self._timer.setInterval(50)
        self._timer.timeout.connect(self._pump_queue)
        self._timer.start()

    def _on_ok(self):
        self._exit_code = 0
        self._stop_event.set()
        QtWidgets.QApplication.instance().quit()

    def _on_cancel(self):
        self._exit_code = 1
        self._stop_event.set()
        QtWidgets.QApplication.instance().quit()

    def closeEvent(self, event):
        self._on_cancel()
        event.accept()

    def _pump_queue(self):
        if self._stop_event.is_set():
            return
        updated = False
        try:
            while True:
                item = self._queue.get_nowait()
                if item is None:
                    break
                self.text.appendPlainText(item.rstrip("\n"))
                updated = True
        except queue.Empty:
            pass
        if updated:
            self.text.verticalScrollBar().setValue(self.text.verticalScrollBar().maximum())

    def enqueue(self, line: str | None):
        self._queue.put(line)

    @property
    def exit_code(self) -> int:
        return self._exit_code


def _run_gui(title: str, ok_label: str, cancel_label: str) -> int:
    app = QtWidgets.QApplication([])
    window = _LogWindow(title, ok_label, cancel_label)
    window.show()

    def reader():
        try:
            for line in sys.stdin:
                window.enqueue(line)
        finally:
            window.enqueue(None)

    reader_thread = threading.Thread(target=reader, daemon=True)
    reader_thread.start()

    app.exec()
    return window.exit_code


def main() -> int:
    parser = argparse.ArgumentParser(description="Deck Game Installer Log Window")
    parser.add_argument("--title", default="Deck Game Installer")
    parser.add_argument("--ok-label", default="OK")
    parser.add_argument("--cancel-label", default="Cancel")
    args = parser.parse_args()

    try:
        return _run_gui(args.title, args.ok_label, args.cancel_label)
    except Exception as exc:
        sys.stderr.write(f"Log window failed: {exc}\n")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
