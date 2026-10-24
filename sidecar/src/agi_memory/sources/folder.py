"""Walks a folder and decides which files the indexer may touch.

Order is deterministic (entries sorted by name). Symlinks and junctions are never followed, so
a link cannot lead the indexer out of the root. Directories the OS refuses to list are recorded
in `unreadable` so the caller can avoid treating their files as deleted.
"""

from __future__ import annotations

import os
from collections.abc import Iterable
from dataclasses import dataclass, field
from pathlib import Path

import pathspec

from agi_memory.config import Settings
from agi_memory.sources.rules import (
    Route,
    is_denied_dir,
    is_denied_file,
    route_for,
    size_limit,
)

_Specs = tuple[
    tuple[str, pathspec.GitIgnoreSpec], ...
]  # (uri prefix the file applies under, rules)


@dataclass(frozen=True, slots=True)
class ScannedFile:
    uri: str  # path relative to the root, '/'-separated
    path: Path
    route: Route
    size: int
    mtime_ns: int


@dataclass(slots=True)
class ScanResult:
    files: list[ScannedFile] = field(default_factory=list)
    denied: int = 0  # deny-list hits (a directory counts once)
    ignored: int = 0  # .gitignore hits and excluded folders (a directory counts once)
    unsupported: int = 0  # regular files of an unknown type
    too_large: int = 0
    unreadable: list[tuple[str, str]] = field(default_factory=list)  # (uri prefix, message)


def scan(root: Path, settings: Settings, exclude: frozenset[str] = frozenset()) -> ScanResult:
    """List the indexable files under `root`.

    `exclude` holds folders to skip, as lower-case '/'-separated paths below the root with a
    trailing slash (`"vendor/skills/"`); each counts once in `ignored`."""
    result = ScanResult()
    _visit(root, "", (), settings, exclude, result)
    return result


def excluded_folders(exclude: Iterable[str]) -> frozenset[str]:
    """The folders to leave out ('/'-separated, relative to the root) in the form `scan` takes."""
    return frozenset(f"{p.strip('/').lower()}/" for p in exclude if p.strip("/"))


def _with_local_gitignore(directory: Path, prefix: str, specs: _Specs) -> _Specs:
    try:
        lines = (directory / ".gitignore").read_text(encoding="utf-8", errors="replace")
    except OSError:  # no .gitignore here (or unreadable): its rules simply do not apply
        return specs
    return (*specs, (prefix, pathspec.GitIgnoreSpec.from_lines(lines.splitlines())))


def _gitignored(uri: str, specs: _Specs) -> bool:
    ignored = False
    for base, spec in specs:
        verdict = spec.check_file(uri[len(base) :]).include
        if verdict is not None:  # deeper files are visited later, so they override
            ignored = verdict
    return ignored


def _visit(
    directory: Path,
    prefix: str,
    specs: _Specs,
    settings: Settings,
    exclude: frozenset[str],
    result: ScanResult,
) -> None:
    specs = _with_local_gitignore(directory, prefix, specs)
    try:
        with os.scandir(directory) as iterator:
            entries = sorted(iterator, key=lambda entry: entry.name)
    except OSError as error:
        result.unreadable.append((prefix, str(error)))
        return
    for entry in entries:
        if entry.is_symlink() or entry.is_junction():
            continue  # never follow a link out of the root
        uri = prefix + entry.name
        if entry.is_dir(follow_symlinks=False):
            if is_denied_dir(entry.name):
                result.denied += 1
            elif (uri + "/").lower() in exclude or _gitignored(uri + "/", specs):
                result.ignored += 1
            else:
                _visit(Path(entry.path), uri + "/", specs, settings, exclude, result)
        elif entry.is_file(follow_symlinks=False):
            _consider_file(entry, uri, specs, settings, result)


def _consider_file(
    entry: os.DirEntry[str], uri: str, specs: _Specs, settings: Settings, result: ScanResult
) -> None:
    if is_denied_file(entry.name):
        result.denied += 1
        return
    if _gitignored(uri, specs):
        result.ignored += 1
        return
    route = route_for(entry.name)
    if route is None:
        result.unsupported += 1
        return
    try:
        stat = entry.stat(follow_symlinks=False)
    except OSError as error:
        result.unreadable.append((uri, str(error)))
        return
    if stat.st_size > size_limit(route, settings):
        result.too_large += 1
        return
    result.files.append(ScannedFile(uri, Path(entry.path), route, stat.st_size, stat.st_mtime_ns))
