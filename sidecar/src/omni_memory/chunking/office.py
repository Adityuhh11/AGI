"""Text of Office files, read with the standard library only.

Word, Excel and PowerPoint (`.docx`, `.xlsx`, `.pptx`) and OpenDocument files (`.odt`, `.ods`,
`.odp`) are zip files of XML. The text comes out one paragraph, row or slide line per line, and is
then chunked like any text file. The older binary formats (`.doc`, `.xls`, `.ppt`) are not read.
A part that unpacks to more than `_MAX_PART` bytes is refused rather than inflated into memory
(a zip bomb).
"""

from __future__ import annotations

import io
import re
import zipfile
from xml.etree import ElementTree

from omni_memory.chunking.common import ChunkingError

_MAX_PART = 32 << 20


def _local(tag: str) -> str:
    return tag.rpartition("}")[2]


def _part(archive: zipfile.ZipFile, name: str) -> ElementTree.Element:
    try:
        with archive.open(name) as part:
            raw = part.read(_MAX_PART + 1)
    except KeyError as error:
        raise ChunkingError(f"the file has no {name}") from error
    if len(raw) > _MAX_PART:
        raise ChunkingError(f"{name} is larger than {_MAX_PART >> 20} MiB unpacked")
    try:
        return ElementTree.fromstring(raw)
    except ElementTree.ParseError as error:
        raise ChunkingError(f"{name} is not readable XML: {error}") from error


def _numbered(names: list[str], prefix: str) -> list[str]:
    """Parts like `ppt/slides/slide10.xml` in number order (a plain sort puts 10 before 2)."""
    found = [(int(m.group(1)), n) for n in names if (m := re.fullmatch(prefix + r"(\d+)\.xml", n))]
    return [name for _, name in sorted(found)]


def _word_paragraphs(root: ElementTree.Element) -> list[str]:
    """One line for each paragraph of a Word or PowerPoint part."""
    lines = []
    for paragraph in root.iter():
        if _local(paragraph.tag) != "p":
            continue
        pieces = []
        for child in paragraph.iter():
            name = _local(child.tag)
            if name == "t":
                pieces.append(child.text or "")
            elif name == "tab":
                pieces.append("	")
            elif name == "br":
                pieces.append(" ")
        if line := "".join(pieces).strip():
            lines.append(line)
    return lines


def _open_document_paragraphs(root: ElementTree.Element) -> list[str]:
    """One line for each paragraph or heading of an OpenDocument file."""
    texts = ("".join(e.itertext()).strip() for e in root.iter() if _local(e.tag) in ("p", "h"))
    return [text for text in texts if text]


def _shared_strings(archive: zipfile.ZipFile) -> list[str]:
    if "xl/sharedStrings.xml" not in archive.namelist():
        return []
    root = _part(archive, "xl/sharedStrings.xml")
    return ["".join(t.text or "" for t in si.iter() if _local(t.tag) == "t") for si in root]


def _sheet_names(archive: zipfile.ZipFile) -> list[str]:
    root = _part(archive, "xl/workbook.xml")
    return [e.get("name", "") for e in root.iter() if _local(e.tag) == "sheet"]


def _sheet_lines(root: ElementTree.Element, strings: list[str]) -> list[str]:
    lines = []
    for row in root.iter():
        if _local(row.tag) != "row":
            continue
        cells = []
        for cell in row:
            if _local(cell.tag) != "c":
                continue
            value = next((c for c in cell if _local(c.tag) in ("v", "is")), None)
            if value is None:
                continue
            if _local(value.tag) == "is":
                text = "".join(t.text or "" for t in value.iter() if _local(t.tag) == "t")
            elif cell.get("t") == "s":
                index = int(value.text) if value.text and value.text.isdigit() else -1
                text = strings[index] if 0 <= index < len(strings) else ""
            else:
                text = value.text or ""
            if text.strip():
                cells.append(text.strip())
        if cells:
            lines.append("\t".join(cells))
    return lines


def _xlsx(archive: zipfile.ZipFile) -> list[str]:
    strings = _shared_strings(archive)
    sheets = _numbered(archive.namelist(), r"xl/worksheets/sheet")
    names = _sheet_names(archive)
    lines: list[str] = []
    for number, sheet in enumerate(sheets):
        # ponytail: sheets are named by position; a workbook edited by hand into another order only
        # gets a wrong heading line.
        if number < len(names) and names[number]:
            lines.append(names[number])
        lines.extend(_sheet_lines(_part(archive, sheet), strings))
    return lines


def office_text(data: bytes, suffix: str, limit: int) -> bytes:
    """The text of an Office file as UTF-8, cut at `limit` bytes. Raises ChunkingError when the
    file is not readable (not a zip, damaged, password-protected, or a part is too large)."""
    try:
        archive = zipfile.ZipFile(io.BytesIO(data))
    except zipfile.BadZipFile as error:
        raise ChunkingError("not a readable Office file (damaged or password-protected)") from error
    with archive:
        try:
            names = archive.namelist()
            if suffix == ".docx":
                lines = _word_paragraphs(_part(archive, "word/document.xml"))
            elif suffix == ".pptx":
                lines = []
                for slide in _numbered(names, r"ppt/slides/slide"):
                    lines.extend(_word_paragraphs(_part(archive, slide)))
            elif suffix == ".xlsx":
                lines = _xlsx(archive)
            else:  # .odt .ods .odp
                lines = _open_document_paragraphs(_part(archive, "content.xml"))
        except (zipfile.BadZipFile, RuntimeError, OSError, EOFError, NotImplementedError) as error:
            raise ChunkingError(f"cannot unpack the file: {error}") from error
    return "\n".join(lines).encode()[:limit]
