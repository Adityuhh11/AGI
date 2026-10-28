"""What the indexer may read: the secret deny list and the file-type routing table.

The deny list is not configurable on purpose. Anything that can hold a credential, plus
dependency and VCS directories, is skipped before it is ever opened.
"""

from __future__ import annotations

import enum
import fnmatch
import sys
from pathlib import PurePosixPath

from omni_memory.config import Settings


class Route(enum.StrEnum):
    """How a file is turned into chunks."""

    CODE = "code"
    MARKDOWN = "markdown"
    TEXT = "text"
    PDF = "pdf"
    IMAGE = "image"
    VIDEO = "video"
    AUDIO = "audio"
    OFFICE = "office"  # a zip of XML: its text is read
    FILE = "file"  # not read at all: found by its name


_DENIED_DIRS = frozenset(
    {".git", "node_modules", ".venv", "venv", "__pycache__", ".ssh", ".docker", "__macosx"}
)
_DENIED_FILE_PATTERNS = (
    ".env*",
    "*.pem",
    "*.key",
    "id_rsa*",
    "id_dsa*",
    "id_ecdsa*",
    "id_ed25519*",
    "*.p12",
    "*.pfx",
    "*.jks",
    "*.keystore",
    ".netrc",
    ".npmrc",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "uv.lock",
    "poetry.lock",
    "pipfile.lock",
    "cargo.lock",
    "go.sum",
    "composer.lock",
    "credentials*.json",
    "client_secret*.json",
    "*service-account*.json",
    "*serviceaccount*.json",
    "secrets.*",
    "*.tfvars",
    "._*",  # macOS resource forks that archives carry along: not real images or text
)

_CODE = frozenset(
    [
        ".js",
        ".jsx",
        ".mjs",
        ".cjs",
        ".ts",
        ".tsx",
        ".py",
        ".go",
        ".rs",
        ".java",
        ".kt",
        ".c",
        ".h",
        ".cpp",
        ".hpp",
        ".cs",
        ".rb",
        ".php",
        ".sh",
        ".ps1",
        ".bat",
        ".sql",
        ".css",
        ".scss",
        ".html",
        ".htm",
        ".svelte",
        ".vue",
        ".lua",
        ".swift",
    ]
)
_MARKDOWN = frozenset({".md", ".markdown"})
_TEXT = frozenset(
    [
        ".txt",
        ".csv",
        ".tsv",
        ".json",
        ".yaml",
        ".yml",
        ".toml",
        ".ini",
        ".cfg",
        ".xml",
        ".rst",
        ".tex",
    ]
)
_IMAGE = frozenset({".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif"})
_VIDEO = frozenset({".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi", ".wmv"})
_AUDIO = frozenset({".mp3", ".wav", ".flac", ".m4a", ".aac", ".ogg", ".opus", ".wma"})
_OFFICE = frozenset({".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp"})
# Files the engine cannot read: only the name is searchable. The older Office formats are binary.
_BY_NAME = frozenset(
    [
        ".doc",
        ".xls",
        ".ppt",
        ".rtf",
        ".epub",
        ".zip",
        ".7z",
        ".rar",
        ".tar",
        ".gz",
        ".tgz",
    ]
)


def is_denied_dir(name: str) -> bool:
    lowered = name.lower()
    return lowered in _DENIED_DIRS or lowered.startswith(".env")  # a venv is often named .env


def is_denied_file(name: str) -> bool:
    lowered = name.lower()
    return any(fnmatch.fnmatchcase(lowered, pattern) for pattern in _DENIED_FILE_PATTERNS)


def is_denied(uri: str) -> bool:
    """True if any directory on the path, or the file name itself, is on the deny list."""
    parts = uri.split("/")
    return any(is_denied_dir(part) for part in parts) or is_denied_file(parts[-1])


def route_for(name: str) -> Route | None:
    """How to chunk a file with this name, or None if it is not a supported type."""
    suffix = PurePosixPath(name).suffix.lower()
    if suffix in _CODE:
        return Route.CODE
    if suffix in _MARKDOWN:
        return Route.MARKDOWN
    if suffix in _TEXT:
        return Route.TEXT
    if suffix == ".pdf":
        return Route.PDF
    if suffix in _IMAGE:
        return Route.IMAGE
    if suffix in _VIDEO:
        return Route.VIDEO
    if suffix in _AUDIO:
        return Route.AUDIO
    if suffix in _OFFICE:
        return Route.OFFICE
    if suffix in _BY_NAME:
        return Route.FILE
    return None


def size_limit(route: Route, settings: Settings) -> int:
    """Largest file (bytes) the route will index. A video, an audio file or a file found by name has
    none: it is never read whole."""
    if route in (Route.VIDEO, Route.AUDIO, Route.FILE):
        return sys.maxsize
    if route in (Route.PDF, Route.IMAGE, Route.OFFICE):
        return settings.max_media_file_bytes
    return settings.max_text_file_bytes
