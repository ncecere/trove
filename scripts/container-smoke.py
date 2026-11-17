#!/usr/bin/env python3
"""Exercise a built image with isolated data and a read-only media fixture.

Usage: python3 scripts/container-smoke.py [image]
Requires Docker and Python's standard library. Removes its containers/volume.
"""

import http.cookiejar
import io
import json
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import zipfile


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True).strip()


def eventually(fn):
    deadline = time.monotonic() + 30
    while True:
        try:
            value = fn()
            if value:
                return value
        except (urllib.error.URLError, ConnectionError):
            pass
        if time.monotonic() >= deadline:
            raise RuntimeError("Timed out waiting for container")
        time.sleep(0.25)


image = sys.argv[1] if len(sys.argv) > 1 else "trove:local"
name = "trove-smoke-" + secrets.token_hex(6)
volume = name + "-data"
password = secrets.token_urlsafe(24)
payload = b"Trove container download fixture\n" * 10
base = ""
client = urllib.request.build_opener(
    urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
)


def request(path, method="GET", body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        base + path, data=data, method=method,
        headers={"X-Requested-With": "trove", "Content-Type": "application/json", **(headers or {})},
    )
    return client.open(req, timeout=5)


def api(path, method="GET", body=None):
    with request("/api/v1" + path, method, body) as response:
        return json.load(response)


def start(media):
    global base
    docker(
        "run", "-d", "--name", name, "--read-only", "--cap-drop=ALL",
        "--security-opt=no-new-privileges:true", "--tmpfs", "/tmp:size=64m,mode=1777",
        "--mount", f"type=volume,src={volume},dst=/data",
        "--mount", f"type=bind,src={media},dst=/media,readonly",
        "-p", "127.0.0.1::8080", "--health-interval=1s", "--health-start-period=1s", image,
    )
    info = json.loads(docker("inspect", name))[0]
    port = info["NetworkSettings"]["Ports"]["8080/tcp"][0]["HostPort"]
    base = f"http://127.0.0.1:{port}"
    eventually(lambda: api("/setup/status"))
    eventually(lambda: docker("inspect", "--format", "{{.State.Health.Status}}", name) == "healthy")
    assert docker("exec", name, "id", "-u") == "10001"
    assert info["HostConfig"]["ReadonlyRootfs"]
    assert not next(m for m in info["Mounts"] if m["Destination"] == "/media")["RW"]


try:
    with tempfile.TemporaryDirectory(prefix="trove-media-") as tmp:
        media = Path(tmp)
        media.chmod(0o755)
        movie = media / "Movies" / "Example (2024)" / "Example.2024.1080p.mp4"
        movie.parent.mkdir(parents=True)
        movie.write_bytes(payload)
        docker("volume", "create", volume)
        start(media)
        assert api("/setup/status")["needsSetup"]
        with request("/") as response:
            html = response.read().decode()
        assert "/assets/" in html and 'id="root"' in html
        api("/setup", "POST", {"username": "container-admin", "password": password})
        api("/settings", "PUT", {"openLibraryEnabled": False, "scanIntervalMinutes": 0})
        api("/libraries", "POST", {"name": "Movies", "type": "movies", "path": "/media/Movies"})
        items = eventually(lambda: api("/items")["items"])
        item = api(f"/items/{items[0]['id']}")
        file_id = item["files"][0]["id"]
        with request(f"/api/v1/files/{file_id}/download") as response:
            assert response.read() == payload
        with request(f"/api/v1/files/{file_id}/download", headers={"Range": "bytes=0-4"}) as response:
            assert response.status == 206 and response.read() == payload[:5]
        with request(f"/api/v1/items/{item['id']}/download") as response:
            archive = zipfile.ZipFile(io.BytesIO(response.read()))
            assert len(archive.namelist()) == 1
            assert archive.read(archive.namelist()[0]) == payload
        with request("/api/v1/auth/logout", "POST") as response:
            assert response.status == 204
        api("/auth/login", "POST", {"username": "container-admin", "password": password})
        assert api("/auth/me")["user"]["role"] == "admin"
        docker("stop", "--time=20", name)
        docker("rm", name)
        client = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
        )
        start(media)
        assert not api("/setup/status")["needsSetup"]
        api("/auth/login", "POST", {"username": "container-admin", "password": password})
        assert len(api("/items")["items"]) == 1
        with request(f"/api/v1/files/{file_id}/download") as response:
            assert response.read() == payload
        assert movie.read_bytes() == payload
        print(f"PASS {image}: embedded UI, health, non-root, setup, login, scan, download, range, ZIP, persistence")
except Exception:
    subprocess.run(["docker", "logs", name], check=False)
    raise
finally:
    subprocess.run(["docker", "rm", "-f", name], capture_output=True)
    subprocess.run(["docker", "volume", "rm", volume], capture_output=True)
