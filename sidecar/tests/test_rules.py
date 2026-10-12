import pytest

from agi_memory.config import Settings
from agi_memory.sources.rules import Route, is_denied, route_for, size_limit

DENIED = [
    ".env",
    "Backend/.env",
    "Frontend/.env.local",
    "Backend/.envsample",
    ".ENV",
    "keys/server.pem",
    "keys/tls.KEY",
    "home/id_rsa",
    "home/id_rsa.pub",
    "home/id_ed25519",
    "cert.p12",
    "cert.pfx",
    "release.keystore",
    ".netrc",
    "package-lock.json",
    "web/yarn.lock",
    "uv.lock",
    "go.sum",
    "node_modules/left-pad/index.js",
    "a/b/node_modules/x.js",
    ".git/config",
    "src/.git/HEAD",
    "pkg/__pycache__/x.pyc",
    ".venv/Lib/x.py",
    "x/.ssh/config",
    "credentials.json",
    "gcp/client_secret_123.json",
    "keys/my-service-account-key.json",
    "config/secrets.yaml",
    "infra/prod.tfvars",
    ".docker/config.json",
    ".env/Lib/site-packages/x.py",
    "app/.env.production/settings.json",
]
ALLOWED = [
    "README.md",
    "src/environment.js",
    "src/keyboard.js",
    "docs/monkey.md",
    "Backend/src/utils/jwt.js",
    "notes/envelope.txt",
    "src/gitignore-notes.md",
    "Week 10 Prep.pdf",
]


@pytest.mark.parametrize("uri", DENIED)
def test_secret_and_noise_paths_are_denied(uri):
    assert is_denied(uri)


@pytest.mark.parametrize("uri", ALLOWED)
def test_ordinary_paths_are_allowed(uri):
    assert not is_denied(uri)


@pytest.mark.parametrize(
    ("name", "route"),
    [
        ("app.js", Route.CODE),
        ("App.JSX", Route.CODE),
        ("main.go", Route.CODE),
        ("style.css", Route.CODE),
        ("README.md", Route.MARKDOWN),
        ("notes.markdown", Route.MARKDOWN),
        ("data.csv", Route.TEXT),
        ("package.json", Route.TEXT),
        ("report.pdf", Route.PDF),
        ("Photo.JPG", Route.IMAGE),
        ("logo.png", Route.IMAGE),
        ("tool.exe", None),
        ("noextension", None),
        (".gitignore", None),
    ],
)
def test_routing_by_extension(name, route):
    assert route_for(name) is route


def test_size_limits_depend_on_the_route(tmp_path):
    settings = Settings(index_dir=tmp_path)
    for route in (Route.CODE, Route.MARKDOWN, Route.TEXT):
        assert size_limit(route, settings) == settings.max_text_file_bytes
    for route in (Route.PDF, Route.IMAGE):
        assert size_limit(route, settings) == settings.max_media_file_bytes
