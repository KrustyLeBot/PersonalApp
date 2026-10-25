"""Per-user settings of the Windows app: which folder holds the data files."""

import json
import os
from pathlib import Path

from . import APP_NAME


def settings_dir() -> Path:
    base = os.environ.get("APPDATA")
    return Path(base) / APP_NAME if base else Path.home() / ".config" / APP_NAME


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
