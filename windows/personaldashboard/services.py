"""What the UI can ask the native side to do. Each public method is one /__bridge/<name> call."""

from typing import Callable

from . import BUILD, VERSION, net, storage, updater
from .config import Config


class BridgeError(Exception):
    """A failure whose message is meant to be shown to the user."""


class Services:
    def __init__(self, config: Config, pick_folder: Callable[[], dict], quit_app: Callable[[], None]):
        self.config = config
        self._pick_folder = pick_folder
        self._quit_app = quit_app
        self._download = updater.Download()

    def info(self, _body):
        return {"build": BUILD, "version": VERSION, "dataDir": str(self.config.data_dir), "hasAccess": True}

    def read_file(self, body):
        try:
            return {"content": storage.read_text(self.config.data_dir, body.get("name", ""))}
        except storage.StorageError as exc:
            raise BridgeError(str(exc)) from exc

    def write_file(self, body):
        try:
            storage.write_text(self.config.data_dir, body.get("name", ""), body.get("content", ""))
        except storage.StorageError as exc:
            raise BridgeError(str(exc)) from exc
        return {}

    def http_get(self, body):
        try:
            return net.http_get(body.get("url", ""), body.get("headers") or {})
        except (ValueError, OSError) as exc:
            raise BridgeError(str(exc)) from exc

    def pick_folder(self, _body):
        return self._pick_folder()

    def download_update(self, body):
        if not updater.running_exe():
            raise BridgeError("La mise à jour automatique ne fonctionne que depuis l'exécutable.")
        try:
            updater.download(body.get("url", ""), int(body.get("size") or 0), body.get("sha256") or "", self._download)
        except (ValueError, OSError) as exc:
            raise BridgeError(str(exc)) from exc
        return {}

    def update_progress(self, _body):
        return self._download.progress()

    def install_update(self, _body):
        exe = updater.running_exe()
        if not exe or not self._download.path:
            raise BridgeError("Aucune mise à jour téléchargée.")
        try:
            updater.install(exe, self._download.path)
            updater.relaunch(exe)
        except OSError as exc:
            raise BridgeError(f"Remplacement de l'exécutable impossible : {exc}") from exc
        self._quit_app()
        return {}

