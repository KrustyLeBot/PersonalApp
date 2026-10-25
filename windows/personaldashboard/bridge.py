"""Local HTTP server: serves the built UI and the /__bridge/<name> calls.

The UI runs in a web view that loads http://127.0.0.1:<port>/?t=<token>. Bridge calls must carry the
token (X-Token) and a local Host header, so another process or a web page in the user's browser cannot
read or write the data files through this port.
"""

import json
import mimetypes
import secrets
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlsplit

from .services import BridgeError, Services

PREFERRED_PORT = 48731  # Fixed so the web view keeps the same origin (and its localStorage) between runs.

MIME = {
    ".html": "text/html; charset=utf-8",
    ".js": "text/javascript; charset=utf-8",
    ".css": "text/css; charset=utf-8",
    ".svg": "image/svg+xml",
    ".json": "application/json",
    ".png": "image/png",
    ".ico": "image/x-icon",
}


class BridgeServer:
    def __init__(self, web_dir: Path, services: Services, port: int = PREFERRED_PORT):
        self.web_dir = Path(web_dir).resolve()
        self.services = services
        self.token = secrets.token_urlsafe(24)
        try:
            self.httpd = ThreadingHTTPServer(("127.0.0.1", port), self._handler())
        except OSError:
            self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), self._handler())
        self.port = self.httpd.server_address[1]
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)

    @property
    def url(self) -> str:
        return f"http://127.0.0.1:{self.port}/?t={self.token}"

    def start(self) -> None:
        self.thread.start()

    def stop(self) -> None:
        self.httpd.shutdown()
        self.httpd.server_close()

    def _handler(self):
        server = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args):
                pass

            def _local_host(self) -> bool:
                return self.headers.get("Host", "") in (f"127.0.0.1:{server.port}", f"localhost:{server.port}")

            def _send(self, status: int, body: bytes, content_type: str, cache: bool = False) -> None:
                self.send_response(status)
                self.send_header("Content-Type", content_type)
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cache-Control", "max-age=3600" if cache else "no-store")
                self.send_header("X-Content-Type-Options", "nosniff")
                self.end_headers()
                self.wfile.write(body)

            def do_GET(self):
                if not self._local_host():
                    return self._send(403, b"forbidden", "text/plain")
                rel = unquote(urlsplit(self.path).path).lstrip("/") or "index.html"
                target = (server.web_dir / rel).resolve()
                if server.web_dir not in target.parents or not target.is_file():
                    return self._send(404, b"not found", "text/plain")
                ctype = MIME.get(target.suffix.lower()) or mimetypes.guess_type(target.name)[0] or "application/octet-stream"
                self._send(200, target.read_bytes(), ctype, cache=target.name != "index.html")

            def do_POST(self):
                path = urlsplit(self.path).path
                if (
                    not path.startswith("/__bridge/")
                    or not self._local_host()
                    or not secrets.compare_digest(self.headers.get("X-Token", ""), server.token)
                ):
                    return self._send(403, b"forbidden", "text/plain")
                name = path[len("/__bridge/"):]
                method = None if name.startswith("_") else getattr(server.services, name, None)
                if not callable(method):
                    return self._send(404, b"unknown call", "text/plain")
                try:
                    length = int(self.headers.get("Content-Length") or 0)
                    body = json.loads(self.rfile.read(length) or b"{}")
                    result = method(body)
                except BridgeError as exc:
                    return self._send(400, str(exc).encode("utf-8"), "text/plain; charset=utf-8")
                except Exception as exc:  # noqa: BLE001 - reported to the UI, which shows it
                    return self._send(500, f"Erreur interne : {exc}".encode("utf-8"), "text/plain; charset=utf-8")
                self._send(200, json.dumps(result).encode("utf-8"), "application/json")

        return Handler
