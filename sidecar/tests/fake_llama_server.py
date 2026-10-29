"""A stand-in for the llama-server program, run by test_llama_server.py through Python.

    --list-devices            prints the real listing saved in tests/data
    otherwise                 serves /health on --port after a short delay

Behaviour is chosen with FAKE_LLAMA_MODE: `ok`, `gpu-fails` (exits unless `--device none`),
`never-ready` (serves nothing), `die` (exits at once on every start).
"""

import http.server
import os
import socketserver
import sys
import time
from pathlib import Path

args = sys.argv[1:]
mode = os.environ.get("FAKE_LLAMA_MODE", "ok")


def option(name: str) -> str:
    return args[args.index(name) + 1]


if "--list-devices" in args:
    print((Path(__file__).parent / "data" / "list_devices_vulkan.txt").read_text())
    sys.exit(0)

print("fake llama-server starting with", " ".join(args), flush=True)
if mode == "die" or (mode == "gpu-fails" and option("--device") != "none"):
    print("error: failed to initialise the device", flush=True)
    sys.exit(3)
if mode == "never-ready":
    time.sleep(60)
    sys.exit(0)

key = option("--api-key")


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args) -> None:
        pass

    def do_GET(self) -> None:
        ok = self.path == "/health" and self.headers.get("Authorization") == "Bearer " + key
        self.send_response(200 if ok else 401)
        self.send_header("Content-Length", "2")
        self.end_headers()
        self.wfile.write(b"{}" if ok else b"no")


class Server(http.server.HTTPServer):
    def server_bind(self) -> None:
        # HTTPServer looks up the host's full name here, which takes many seconds on a Mac without DNS.
        socketserver.TCPServer.server_bind(self)
        self.server_name, self.server_port = "127.0.0.1", self.server_address[1]


time.sleep(0.3)
Server(("127.0.0.1", int(option("--port"))), Handler).serve_forever()
