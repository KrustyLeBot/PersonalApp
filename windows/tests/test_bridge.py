import json
import tempfile
import unittest
import urllib.error
import urllib.request
from pathlib import Path

from personaldashboard.bridge import BridgeServer
from personaldashboard.config import Config
from personaldashboard.services import Services
from personaldashboard import storage


def make_server(tmp: Path):
    web = tmp / "web"
    web.mkdir()
    (web / "index.html").write_text("<html>ok</html>", encoding="utf-8")
    (web / "app.js").write_text("console.log(1)", encoding="utf-8")
    (tmp / "secret.txt").write_text("secret", encoding="utf-8")
    config = Config(tmp / "data", path=tmp / "config.json")
    services = Services(config, lambda: {"changed": False, "dataDir": str(config.data_dir)}, lambda: None)
    server = BridgeServer(web, services, port=0)
    server.start()
    return server, config


def call(server, name, body=None, token=None):
    req = urllib.request.Request(
        f"http://127.0.0.1:{server.port}/__bridge/{name}",
        data=json.dumps(body or {}).encode(),
        headers={"X-Token": server.token if token is None else token, "Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


class BridgeTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp = Path(self._tmp.name)
        self.server, self.config = make_server(self.tmp)

    def tearDown(self):
        self.server.stop()
        self._tmp.cleanup()

    def test_serves_static_files_and_blocks_traversal(self):
        base = f"http://127.0.0.1:{self.server.port}"
        with urllib.request.urlopen(base + "/") as resp:
            self.assertEqual(resp.read(), b"<html>ok</html>")
        with urllib.request.urlopen(base + "/app.js") as resp:
            self.assertTrue(resp.headers["Content-Type"].startswith("text/javascript"))
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            urllib.request.urlopen(base + "/../secret.txt")
        self.assertEqual(ctx.exception.code, 404)
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            urllib.request.urlopen(base + "/%2e%2e/secret.txt")
        self.assertEqual(ctx.exception.code, 404)

    def test_bridge_requires_token(self):
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            call(self.server, "info", token="wrong")
        self.assertEqual(ctx.exception.code, 403)

    def test_bridge_rejects_foreign_host(self):
        req = urllib.request.Request(
            f"http://127.0.0.1:{self.server.port}/__bridge/info",
            data=b"{}",
            headers={"X-Token": self.server.token, "Host": "evil.example"},
            method="POST",
        )
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            urllib.request.urlopen(req)
        self.assertEqual(ctx.exception.code, 403)

    def test_info(self):
        info = call(self.server, "info")
        self.assertEqual(info["dataDir"], str(self.config.data_dir))
        self.assertTrue(info["hasAccess"])
        self.assertIn("build", info)

    def test_write_then_read_roundtrip_creates_folder(self):
        self.assertIsNone(call(self.server, "read_file", {"name": "walk.json"})["content"])
        call(self.server, "write_file", {"name": "walk.json", "content": '{"entries": {"2025-01-01": 4.5}}\n'})
        self.assertEqual(call(self.server, "read_file", {"name": "walk.json"})["content"], '{"entries": {"2025-01-01": 4.5}}\n')
        self.assertEqual(sorted(p.name for p in self.config.data_dir.iterdir()), ["walk.json"])  # no .tmp left behind

    def test_unicode_content(self):
        call(self.server, "write_file", {"name": "portfolio.json", "content": "Prêt immobilier – Noël €"})
        self.assertEqual(call(self.server, "read_file", {"name": "portfolio.json"})["content"], "Prêt immobilier – Noël €")

    def test_rejects_unsafe_file_names(self):
        for name in ("../x.json", "a/b.json", "..", "", "x y.json"):
            with self.assertRaises(urllib.error.HTTPError) as ctx:
                call(self.server, "read_file", {"name": name})
            self.assertEqual(ctx.exception.code, 400, name)

    def test_http_get_only_allows_https(self):
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            call(self.server, "http_get", {"url": "http://example.com"})
        self.assertEqual(ctx.exception.code, 400)

    def test_private_methods_and_unknown_calls_are_not_exposed(self):
        for name in ("__init__", "_pick_folder", "config", "nope"):
            with self.assertRaises(urllib.error.HTTPError) as ctx:
                call(self.server, name)
            self.assertEqual(ctx.exception.code, 404, name)

    def test_update_requires_frozen_exe(self):
        with self.assertRaises(urllib.error.HTTPError) as ctx:
            call(self.server, "download_update", {"url": "https://example.com/x.exe", "size": 1})
        self.assertEqual(ctx.exception.code, 400)
        self.assertEqual(call(self.server, "update_progress"), {"done": 0, "total": 0})


class ConfigTests(unittest.TestCase):
    def test_data_dir_is_persisted(self):
        with tempfile.TemporaryDirectory() as t:
            path = Path(t) / "cfg" / "config.json"
            cfg = Config(Path(t) / "default", path=path)
            self.assertEqual(cfg.data_dir, Path(t) / "default")
            cfg.set_data_dir(Path(t) / "elsewhere")
            self.assertEqual(Config(Path(t) / "default", path=path).data_dir, Path(t) / "elsewhere")

    def test_has_data(self):
        with tempfile.TemporaryDirectory() as t:
            self.assertFalse(storage.has_data(Path(t)))
            (Path(t) / "walk.json").write_text("{}")
            self.assertTrue(storage.has_data(Path(t)))


if __name__ == "__main__":
    unittest.main()
