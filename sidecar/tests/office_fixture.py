"""Small Office and OpenDocument files built from XML, for tests."""

import io
import zipfile
from xml.sax.saxutils import escape

W = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
A = "http://schemas.openxmlformats.org/drawingml/2006/main"
S = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
ODF = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"


def make_zip(parts: dict[str, str | bytes]) -> bytes:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, body in parts.items():
            archive.writestr(name, body)
    return buffer.getvalue()


def make_docx(paragraphs: list[str]) -> bytes:
    body = "".join(f"<w:p><w:r><w:t>{escape(p)}</w:t></w:r></w:p>" for p in paragraphs)
    document = f'<w:document xmlns:w="{W}"><w:body>{body}</w:body></w:document>'
    return make_zip({"word/document.xml": document})


def make_pptx(slides: dict[int, list[str]]) -> bytes:
    parts = {}
    for number, lines in slides.items():
        body = "".join(f"<a:p><a:r><a:t>{escape(t)}</a:t></a:r></a:p>" for t in lines)
        parts[f"ppt/slides/slide{number}.xml"] = f'<p:sld xmlns:a="{A}" xmlns:p="p">{body}</p:sld>'
    return make_zip(parts)


def make_xlsx(sheets: dict[str, list[list[str | int]]]) -> bytes:
    strings: list[str] = []
    parts: dict[str, str | bytes] = {}
    names = ""
    for number, (name, rows) in enumerate(sheets.items(), start=1):
        names += f'<sheet name="{escape(name)}" sheetId="{number}"/>'
        xml = ""
        for row in rows:
            cells = ""
            for value in row:
                if isinstance(value, int):
                    cells += f"<c><v>{value}</v></c>"
                else:
                    strings.append(value)
                    cells += f'<c t="s"><v>{len(strings) - 1}</v></c>'
            xml += f"<row>{cells}</row>"
        parts[f"xl/worksheets/sheet{number}.xml"] = (
            f'<worksheet xmlns="{S}"><sheetData>{xml}</sheetData></worksheet>'
        )
    parts["xl/workbook.xml"] = f'<workbook xmlns="{S}"><sheets>{names}</sheets></workbook>'
    items = "".join(f"<si><t>{escape(s)}</t></si>" for s in strings)
    parts["xl/sharedStrings.xml"] = f'<sst xmlns="{S}">{items}</sst>'
    return make_zip(parts)


def make_odt(paragraphs: list[str]) -> bytes:
    body = "".join(f"<text:p>{escape(p)}</text:p>" for p in paragraphs)
    content = (
        f'<office:document-content xmlns:text="{ODF}" xmlns:office="o">{body}'
        "</office:document-content>"
    )
    return make_zip({"content.xml": content})
