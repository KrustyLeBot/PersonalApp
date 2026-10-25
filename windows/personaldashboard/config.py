"""Per-user settings of the Windows app: which folder holds the data files."""

import json
import os
import sys
from pathlib import Path

from . import APP_NAME


def settings_dir() -> Path:
    base = os.environ.get("APPDATA")
    return Path(base) / APP_NAME if base else Path.home() / ".config" / APP_NAME


def documents_dir() -> Path:
    """The user's Documents folder (follows a OneDrive redirection on Windows)."""
    if sys.platform == "win32":
        try:
            import ctypes
            from ctypes import wintypes

            class Guid(ctypes.Structure):
                _fields_ = [
                    ("Data1", wintypes.DWORD),
                    ("Data2", wintypes.WORD),
                    ("Data3", wintypes.WORD),
                    ("Data4", ctypes.c_ubyte * 8),
                ]

            folderid_documents = Guid(
                0xFDD39AD0, 0x238F, 0x46AF, (ctypes.c_ubyte * 8)(0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7)
            )
            path = ctypes.c_wchar_p()
            if ctypes.windll.shell32.SHGetKnownFolderPath(ctypes.byref(folderid_documents), 0, None, ctypes.byref(path)) == 0:
                result = path.value
                ctypes.windll.ole32.CoTaskMemFree(path)
                if result:
                    return Path(result)
        except (OSError, AttributeError):
            pass
    return Path.home() / "Documents"


class Config:
    def __init__(self, default_data_dir: Path, path: Path | None = None):
        self.path = path or settings_dir() / "config.json"
        self.default_data_dir = Path(default_data_dir)
        self._data_dir = self.default_data_dir
        self._load()

    def _load(self) -> None:
        try:
            raw = json.loads(self.path.read_text(encoding="utf-8"))
            chosen = raw.get("data_dir")
            if chosen:
                self._data_dir = Path(chosen)
        except (OSError, ValueError):
            pass

    @property
    def data_dir(self) -> Path:
        return self._data_dir

    def set_data_dir(self, folder: Path) -> None:
        self._data_dir = Path(folder)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps({"data_dir": str(self._data_dir)}, indent=2), encoding="utf-8")
