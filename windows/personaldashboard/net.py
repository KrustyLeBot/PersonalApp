"""Outgoing HTTP for the UI (market data, update check): no CORS in a native client."""

import urllib.error
import urllib.request

MAX_BODY = 20 * 1024 * 1024


def http_get(url: str, headers: dict | None = None, timeout: int = 30) -> dict:
    if not url.startswith("https://"):
        raise ValueError("Seules les URL https sont autorisées")
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return {"status": resp.status, "body": resp.read(MAX_BODY).decode("utf-8", "replace")}
    except urllib.error.HTTPError as err:
        return {"status": err.code, "body": err.read(MAX_BODY).decode("utf-8", "replace")}
    except (urllib.error.URLError, OSError) as exc:
        raise OSError(f"Requête impossible : {exc}") from exc
