"""Self-update: download the new exe with an integrity check, swap it in place of the running one.

Same approach as GitEnough: Windows refuses to overwrite a running executable but lets it be renamed,
so the old exe becomes <name>.old (removed at the next start) and the new one takes its name.
"""

import hashlib
import os
import subprocess
import sys
import threading
import urllib.request

from . import APP_NAME, BUILD

USER_AGENT = f"{APP_NAME}/{BUILD}"


def running_exe() -> str | None:
    """Path of the running exe, or None when running from source."""
    return sys.executable if getattr(sys, "frozen", False) else None


class Download:
    """Thread-safe progress of the current download."""

    def __init__(self):
        self._lock = threading.Lock()
        self._done = 0
        self._total = 0
        self.path: str | None = None

    def set(self, done: int, total: int) -> None:
        with self._lock:
            self._done, self._total = done, total

    def progress(self) -> dict:
        with self._lock:
            return {"done": self._done, "total": self._total}


def download(url: str, size: int, sha256: str, state: Download, dest_dir: str) -> str:
    """Download next to the running exe and verify it; returns its path.

    It must be on the same drive as the exe: swapping files across drives fails on Windows (WinError 17).
    A truncated or tampered file never survives.
    """
    if not url.startswith("https://"):
        raise ValueError("Seules les URL https sont autorisées")
    target = os.path.join(dest_dir, f"{APP_NAME}-update.exe.download")
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    digest, done = hashlib.sha256(), 0
    state.set(0, size)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp, open(target, "wb") as out:
            total = int(resp.headers.get("Content-Length") or size or 0)
            while True:
                chunk = resp.read(256 * 1024)
                if not chunk:
                    break
                out.write(chunk)
                digest.update(chunk)
                done += len(chunk)
                state.set(done, total)
        if size and done != size:
            raise OSError(f"Téléchargement incomplet ({done} sur {size} octets)")
        if sha256 and digest.hexdigest() != sha256.lower():
            raise OSError("Le fichier téléchargé ne correspond pas au SHA-256 publié")
        with open(target, "rb") as f:
            if f.read(2) != b"MZ":
                raise OSError("Le fichier téléchargé n'est pas un exécutable Windows")
    except BaseException:
        try:
            os.remove(target)
        except OSError:
            pass
        raise
    state.path = target
    return target


def install(current_exe: str, new_file: str) -> None:
    old = current_exe + ".old"
    if os.path.exists(old):
        os.remove(old)
    os.replace(current_exe, old)
    try:
        os.replace(new_file, current_exe)
    except OSError:
        os.replace(old, current_exe)  # Put the working version back.
        raise


def relaunch(exe: str) -> None:
    # A onefile exe started by another one inherits its bootloader variables and would load its files
    # from the parent's temp folder, deleted as the parent exits. The reset makes it unpack its own.
    env = {k: v for k, v in os.environ.items() if not k.startswith(("_PYI", "_MEIPASS"))}
    env["PYINSTALLER_RESET_ENVIRONMENT"] = "1"
    flags = subprocess.DETACHED_PROCESS | subprocess.CREATE_NEW_PROCESS_GROUP
    subprocess.Popen([exe], creationflags=flags, close_fds=True, cwd=os.path.dirname(exe), env=env)


def cleanup_previous() -> bool:
    """Remove the exe left behind by the last self-update; True once nothing is left.

    Right after an update the previous process is still exiting and keeps the .old file locked for a
    few seconds, so callers retry until this returns True.
    """
    exe = running_exe()
    old = exe + ".old" if exe else ""
    if not old or not os.path.exists(old):
        return True
    try:
        os.remove(old)
        return True
    except OSError:
        return False
