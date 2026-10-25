"""Qt shell: one window hosting the web UI, plus the native pieces the UI cannot do itself."""

import os
import shutil
import sys
import tempfile
import time
from concurrent.futures import Future
from pathlib import Path

from PySide6.QtCore import QLockFile, QObject, QStandardPaths, QTimer, QUrl, Signal, Slot
from PySide6.QtGui import QColor, QIcon
from PySide6.QtWebEngineCore import QWebEnginePage, QWebEngineProfile
from PySide6.QtWebEngineWidgets import QWebEngineView
from PySide6.QtWidgets import QApplication, QFileDialog, QMainWindow, QMessageBox

from . import APP_NAME, updater
from .bridge import BridgeServer
from .config import Config, settings_dir
from .services import Services
from .storage import DATA_FILES, has_data

BG = "#0f172a"
EXTRA_FILES = ("cache.json", "manifest.json")


def resource_dir() -> Path:
    base = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent.parent))
    return base / "personaldashboard"


class Gui(QObject):
    """Runs on the GUI thread what the bridge threads ask for (dialogs, quitting)."""

    pick_requested = Signal(object)
    quit_requested = Signal()

    def __init__(self, window: QMainWindow, config: Config):
        super().__init__(window)
        self.window, self.config = window, config
        self.pick_requested.connect(self._pick)

    @Slot(object)
    def _pick(self, result: Future) -> None:
        try:
            result.set_result(self._choose_folder())
        except BaseException as exc:  # noqa: BLE001 - handed back to the calling bridge thread
            result.set_exception(exc)

    def _choose_folder(self) -> dict:
        current = self.config.data_dir
        start = current if current.exists() else current.parent
        chosen = QFileDialog.getExistingDirectory(self.window, "Dossier des données", str(start))
        if not chosen or Path(chosen) == current:
            return {"changed": False, "dataDir": str(current)}

        new = Path(chosen)
        if has_data(current) and not has_data(new):
            answer = QMessageBox.question(
                self.window,
                "Copier les données ?",
                f"Le dossier choisi ne contient pas de données.\nCopier les fichiers actuels vers :\n{new} ?",
                QMessageBox.Yes | QMessageBox.No,
                QMessageBox.Yes,
            )
            if answer == QMessageBox.Yes:
                for name in (*DATA_FILES, *EXTRA_FILES):
                    if (current / name).is_file():
                        shutil.copy2(current / name, new / name)
        self.config.set_data_dir(new)
        return {"changed": True, "dataDir": str(new)}


def acquire_single_instance() -> QLockFile | None:
    """A second launch exits, but waits a while: after a self-update the old process is still closing."""
    lock = QLockFile(os.path.join(tempfile.gettempdir(), f"{APP_NAME}.lock"))
    lock.setStaleLockTime(30000)
    deadline = time.monotonic() + 15
    while not lock.tryLock(200):
        if time.monotonic() > deadline:
            return None
    return lock


def run() -> int:
    if sys.platform == "win32":
        import ctypes

        ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID(f"KrustyLeBot.{APP_NAME}")

    app = QApplication(sys.argv)
    app.setApplicationName(APP_NAME)
    icon = QIcon(str(resource_dir() / "assets" / "icon.ico"))
    app.setWindowIcon(icon)

    lock = acquire_single_instance()
    if lock is None:
        return 0

    documents = QStandardPaths.writableLocation(QStandardPaths.DocumentsLocation)
    config = Config(Path(documents) / APP_NAME)

    window = QMainWindow()
    window.setWindowTitle(APP_NAME)
    window.setWindowIcon(icon)
    window.resize(1100, 820)
    window.setMinimumSize(360, 520)

    gui = Gui(window, config)

    def pick_folder() -> dict:
        result: Future = Future()
        gui.pick_requested.emit(result)
        return result.result(timeout=600)

    services = Services(config, pick_folder, gui.quit_requested.emit)
    server = BridgeServer(resource_dir() / "web", services)
    server.start()

    # An explicit profile keeps localStorage (e.g. the "confidential" display mode) between runs.
    web_dir = settings_dir() / "webdata"
    profile = QWebEngineProfile(APP_NAME, window)
    profile.setPersistentStoragePath(str(web_dir))
    profile.setCachePath(str(web_dir / "cache"))
    page = QWebEnginePage(profile, window)
    page.setBackgroundColor(QColor(BG))
    view = QWebEngineView(window)
    view.setPage(page)
    window.setCentralWidget(view)
    view.load(QUrl(server.url))
    window.show()

    gui.quit_requested.connect(lambda: QTimer.singleShot(300, app.quit))

    # Right after a self-update the previous exe is still locked for a few seconds.
    cleanup = QTimer(window)
    cleanup.setInterval(1000)
    tries = {"n": 0}

    def try_cleanup():
        tries["n"] += 1
        if updater.cleanup_previous() or tries["n"] >= 60:
            cleanup.stop()

    cleanup.timeout.connect(try_cleanup)
    cleanup.start()

    code = app.exec()
    server.stop()
    lock.unlock()
    return code
