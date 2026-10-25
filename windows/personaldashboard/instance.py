"""Single-instance guard (a named Windows mutex, released by the OS when the process ends)."""

import sys
import time


class SingleInstance:
    def __init__(self, name: str):
        self.name = name
        self._handle = None

    def acquire(self, wait: float = 15.0) -> bool:
        """A second launch exits, but waits a while: after a self-update the old process is still closing."""
        if sys.platform != "win32":
            return True

        import ctypes
        from ctypes import wintypes

        error_already_exists = 183
        kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel32.CreateMutexW.restype = wintypes.HANDLE
        kernel32.CreateMutexW.argtypes = [wintypes.LPVOID, wintypes.BOOL, wintypes.LPCWSTR]
        kernel32.CloseHandle.argtypes = [wintypes.HANDLE]

        deadline = time.monotonic() + wait
        while True:
            handle = kernel32.CreateMutexW(None, False, f"Local\\{self.name}")
            if handle and ctypes.get_last_error() != error_already_exists:
                self._handle = handle
                return True
            if handle:
                kernel32.CloseHandle(handle)
            if time.monotonic() > deadline:
                return False
            time.sleep(0.2)
