"""Reading and writing the data files (same JSON files as the Android app)."""

import os
import re
from pathlib import Path

DATA_FILES = ("portfolio.json", "telework.json", "walk.json")
_SAFE_NAME = re.compile(r"^[A-Za-z0-9_.-]+$")


class StorageError(Exception):
    pass


def _target(folder: Path, name: str) -> Path:
    if not _SAFE_NAME.match(name) or ".." in name:
        raise StorageError("Nom de fichier invalide")
    return Path(folder) / name


def read_text(folder: Path, name: str) -> str | None:
    path = _target(folder, name)
    try:
        return path.read_text(encoding="utf-8")
    except FileNotFoundError:
        return None
    except OSError as exc:
        raise StorageError(f"Lecture impossible : {exc}") from exc


def write_text(folder: Path, name: str, content: str) -> None:
    """Written to a temporary file then swapped in, so a crash never leaves a half-written file."""
    path = _target(folder, name)
    tmp = path.with_name(path.name + ".tmp")
    try:
        Path(folder).mkdir(parents=True, exist_ok=True)
        with open(tmp, "w", encoding="utf-8", newline="\n") as out:
            out.write(content)
            out.flush()
            os.fsync(out.fileno())
        os.replace(tmp, path)
    except OSError as exc:
        try:
            tmp.unlink()
        except OSError:
            pass
        raise StorageError(f"Écriture impossible : {exc}") from exc


def has_data(folder: Path) -> bool:
    return any((Path(folder) / name).is_file() for name in DATA_FILES)
