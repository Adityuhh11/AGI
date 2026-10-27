import os

import pytest

from omni_memory.config import Settings
from omni_memory.sources.folder import scan
from omni_memory.sources.rules import Route


def write(root, relative, content="x"):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path / "idx")


def uris(result):
    return [f.uri for f in result.files]


def test_denied_entries_are_counted_and_never_listed(tmp_path, settings):
    root = tmp_path / "src"
    for name in (
        ".env",
        ".git/config",
        "node_modules/pkg/index.js",
        "server.pem",
        "package-lock.json",
    ):
        write(root, name)
    write(root, "ok.js")
    result = scan(root, settings)
    assert uris(result) == ["ok.js"]
    assert result.denied == 5  # three files plus two directories counted once each


def test_nested_gitignore_rules_apply_and_deeper_files_override(tmp_path, settings):
    root = tmp_path / "src"
    write(root, ".gitignore", "*.md\nbuild/\n/secret.txt\n")
    write(root, "sub/.gitignore", "!keep.md\n")
    for name in (
        "app.js", "notes.md", "secret.txt", "build/out.js", "sub/keep.md", "sub/other.md",
        "sub/deep/x.js", "other/secret.txt",
    ):  # fmt: skip
        write(root, name)
    result = scan(root, settings)
    assert uris(result) == ["app.js", "other/secret.txt", "sub/deep/x.js", "sub/keep.md"]
    assert result.ignored == 4  # notes.md, secret.txt, build/ (once), sub/other.md


def test_unsupported_and_oversized_files_are_counted(tmp_path):
    settings = Settings(index_dir=tmp_path / "idx", max_text_file_bytes=10, max_media_file_bytes=10)
    root = tmp_path / "src"
    write(root, "data.bin", "binary-ish")
    write(root, "small.txt", "tiny")
    write(root, "big.txt", "x" * 100)
    (root / "pic.png").write_bytes(b"x" * 20)
    result = scan(root, settings)
    assert uris(result) == ["small.txt"]
    assert (result.unsupported, result.too_large) == (1, 2)


def test_order_is_sorted_and_uris_use_forward_slashes_with_odd_names(tmp_path, settings):
    root = tmp_path / "src"
    for name in ("my notes/café.md", "a b.txt", "Z.js", "a.js"):
        write(root, name)
    result = scan(root, settings)
    assert uris(result) == ["Z.js", "a b.txt", "a.js", "my notes/café.md"]
    assert all("\\" not in f.uri for f in result.files)


def test_file_records_size_mtime_and_route(tmp_path, settings):
    root = tmp_path / "src"
    path = write(root, "a/b.md", "hello")
    (found,) = scan(root, settings).files
    stat = path.stat()
    assert (found.uri, found.route, found.size, found.mtime_ns) == (
        "a/b.md", Route.MARKDOWN, 5, stat.st_mtime_ns,
    )  # fmt: skip
    assert found.path == path


def test_symlinks_are_not_followed(tmp_path, settings):
    root = tmp_path / "src"
    outside = tmp_path / "outside"
    write(outside, "secret.txt", "do not index")
    write(root, "real.txt")
    try:
        os.symlink(outside, root / "link", target_is_directory=True)
        os.symlink(outside / "secret.txt", root / "file-link.txt")
    except OSError:
        pytest.skip("creating symlinks needs privileges on this machine")
    assert uris(scan(root, settings)) == ["real.txt"]


def test_unreadable_directory_is_reported_and_the_rest_still_scanned(
    tmp_path, settings, monkeypatch
):
    root = tmp_path / "src"
    write(root, "ok.js")
    write(root, "locked/hidden.js")
    real_scandir = os.scandir

    def flaky(path):
        if str(path).endswith("locked"):
            raise PermissionError("access denied")
        return real_scandir(path)

    monkeypatch.setattr(os, "scandir", flaky)
    result = scan(root, settings)
    assert uris(result) == ["ok.js"]
    assert result.unreadable == [("locked/", "access denied")]


def test_missing_root_is_reported_as_unreadable_root(tmp_path, settings):
    result = scan(tmp_path / "nope", settings)
    assert result.files == []
    assert [prefix for prefix, _ in result.unreadable] == [""]


def test_excluded_folders_are_skipped_by_exact_path_and_any_case(tmp_path, settings):
    root = tmp_path / "src"
    for name in (
        "keep.txt",
        "Vendor/Skills/a.txt",
        "Vendor/Skills/deep/b.txt",
        "Vendor/Skills-extra/c.txt",
        "other/Vendor/Skills/d.txt",
    ):
        write(root, name)
    result = scan(root, settings, frozenset({"vendor/skills/"}))
    assert uris(result) == ["Vendor/Skills-extra/c.txt", "keep.txt", "other/Vendor/Skills/d.txt"]
    assert result.ignored == 1  # the folder counts once
