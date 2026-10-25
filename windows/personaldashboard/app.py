"""Native shell: one window showing the web UI (the system's Edge WebView2), plus the native pieces
the UI cannot do itself (folder dialog, quitting)."""

import ctypes
import shutil
import sys
import threading
import time
from pathlib import Path

import webview

from . import APP_NAME, updater
from .bridge import BridgeServer
from .config import Config, documents_dir, settings_dir
from .instance import SingleInstance
from .services import Services
from .storage import DATA_FILES, has_data

BG = "#0f172a"
EXTRA_FILES = ("cache.json", "manifest.json")
WEBVIEW2_URL = "https://developer.microsoft.com/microsoft-edge/webview2/"


def resource_dir() -> Path:
    base = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent.parent))
    return base / "personaldashboard"


class Gui:
    """Dialogs and quitting, called from the bridge threads."""

    def __init__(self, config: Config):
        self.config = config
        self.window: "webview.Window | None" = None

    def pick_folder(self) -> dict:
        current = self.config.data_dir
        start = current if current.exists() else current.parent
        chosen = self.window.create_file_dialog(webview.FileDialog.FOLDER, directory=str(start))
        if not chosen or Path(chosen[0]) == current:
            return {"changed": False, "dataDir": str(current)}

        new = Path(chosen[0])
        if has_data(current) and not has_data(new):
            copy = self.window.create_confirmation_dialog(
                "Copier les données ?",
                f"Le dossier choisi ne contient pas de données.\nCopier les fichiers actuels vers :\n{new} ?",
            )
            if copy:
                for name in (*DATA_FILES, *EXTRA_FILES):
                    if (current / name).is_file():
                        shutil.copy2(current / name, new / name)
        self.config.set_data_dir(new)
        return {"changed": True, "dataDir": str(new)}

    def quit(self) -> None:
        # Delayed so the bridge call that triggered it can still answer the UI.
        threading.Timer(0.3, self.window.destroy).start()


def cleanup_previous_exe() -> None:
    """Right after a self-update the previous exe is still locked for a few seconds."""
    for _ in range(60):
        if updater.cleanup_previous():
            return
        time.sleep(1)


def show_error(message: str) -> None:
    if sys.platform == "win32":
        ctypes.windll.user32.MessageBoxW(None, message, APP_NAME, 0x10)
    else:
        print(message, file=sys.stderr)


def run() -> int:
    if sys.platform == "win32":
        ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID(f"KrustyLeBot.{APP_NAME}")

    instance = SingleInstance(APP_NAME)
    if not instance.acquire():
        return 0

    config = Config(documents_dir() / APP_NAME)
    gui = Gui(config)
    services = Services(config, gui.pick_folder, gui.quit)
    server = BridgeServer(resource_dir() / "web", services)
    server.start()

    gui.window = webview.create_window(
        APP_NAME, server.url, width=1100, height=820, min_size=(360, 520), background_color=BG
    )
    try:
        # private_mode=False + storage_path keep localStorage (e.g. the "confidential" mode) between runs.
        webview.start(
            cleanup_previous_exe,
            gui="edgechromium",
            private_mode=False,
            storage_path=str(settings_dir() / "webdata"),
            icon=str(resource_dir() / "assets" / "icon.ico"),
        )
    except Exception as exc:  # noqa: BLE001 - shown to the user: usually a missing WebView2 runtime
        show_error(
            f"{APP_NAME} n'a pas pu ouvrir sa fenêtre.\n\n{exc}\n\n"
            f"Il faut le composant Microsoft Edge WebView2 (inclus dans Windows 11) :\n{WEBVIEW2_URL}"
        )
        return 1
    finally:
        server.stop()
    return 0
