import pytest

from office_fixture import make_docx, make_odt, make_pptx, make_xlsx, make_zip
from omni_memory.chunking.common import ChunkingError
from omni_memory.chunking.office import _MAX_PART, office_text

BIG = 1 << 20


def text(data: bytes, suffix: str, limit: int = BIG) -> list[str]:
    return office_text(data, suffix, limit).decode().splitlines()


def test_a_word_document_gives_one_line_per_paragraph():
    data = make_docx(["Quarterly budget", "", "Rent & utilities: 1200 <approved>"])
    assert text(data, ".docx") == ["Quarterly budget", "Rent & utilities: 1200 <approved>"]


def test_tabs_and_line_breaks_in_a_paragraph_stay_on_its_line():
    xml = (
        '<w:document xmlns:w="w"><w:p><w:r><w:t>a</w:t><w:tab/><w:t>b</w:t><w:br/>'
        "<w:t>c</w:t></w:r></w:p></w:document>"
    )
    assert text(make_zip({"word/document.xml": xml}), ".docx") == ["a\tb c"]


def test_text_in_any_language_survives():
    assert text(make_docx(["résumé 履歴書 Привет"]), ".docx") == ["résumé 履歴書 Привет"]


def test_slides_come_in_number_order_not_name_order():
    data = make_pptx({10: ["ten"], 2: ["two"], 1: ["one"]})
    assert text(data, ".pptx") == ["one", "two", "ten"]


def test_a_workbook_gives_each_sheet_its_name_and_a_line_per_row():
    data = make_xlsx(
        {"Costs": [["item", "price"], ["rent", 1200]], "Notes": [["remember the deposit"]]}
    )
    assert text(data, ".xlsx") == [
        "Costs",
        "item\tprice",
        "rent\t1200",
        "Notes",
        "remember the deposit",
    ]


def test_empty_cells_and_blank_rows_leave_no_lines():
    assert text(make_xlsx({"S": [[], [" "], ["x"]]}), ".xlsx") == ["S", "x"]


def sheet_only(row: str) -> bytes:
    sheet = f'<worksheet xmlns="s"><sheetData><row>{row}</row></sheetData></worksheet>'
    return make_zip({"xl/worksheets/sheet1.xml": sheet, "xl/workbook.xml": "<workbook/>"})


def test_a_cell_that_points_past_the_shared_strings_is_empty_not_an_error():
    data = sheet_only('<c t="s"><v>99</v></c><c t="s"><v>x</v></c>')
    assert text(data, ".xlsx") == []


def test_inline_strings_are_read():
    assert text(sheet_only('<c t="inlineStr"><is><t>inline</t></is></c>'), ".xlsx") == ["inline"]


@pytest.mark.parametrize("suffix", [".odt", ".ods", ".odp"])
def test_open_document_files_are_read_from_content_xml(suffix):
    assert text(make_odt(["first", "second"]), suffix) == ["first", "second"]


def test_a_document_with_no_text_gives_nothing():
    assert office_text(make_docx([]), ".docx", BIG) == b""


def test_the_text_is_cut_at_the_limit():
    assert len(office_text(make_docx(["x" * 1000]), ".docx", 100)) == 100


@pytest.mark.parametrize(
    "data", [b"", b"not a zip at all", b"\xd0\xcf\x11\xe0" + bytes(600), b"PK\x03\x04"]
)
def test_a_file_that_is_not_a_zip_is_refused(data):
    with pytest.raises(ChunkingError, match="damaged or password-protected"):
        office_text(data, ".docx", BIG)


def test_a_truncated_zip_is_refused():
    with pytest.raises(ChunkingError):
        office_text(make_docx(["hello"] * 50)[:-40], ".docx", BIG)


def test_a_zip_without_the_expected_part_is_refused():
    with pytest.raises(ChunkingError, match=r"no word/document.xml"):
        office_text(make_zip({"other.txt": "x"}), ".docx", BIG)


def test_broken_xml_is_refused():
    with pytest.raises(ChunkingError, match="not readable XML"):
        office_text(make_zip({"word/document.xml": "<w:p>"}), ".docx", BIG)


def test_a_part_that_unpacks_huge_is_refused_without_being_inflated():
    bomb = make_zip({"word/document.xml": "<a>" + "x" * (_MAX_PART + 10) + "</a>"})
    assert len(bomb) < 1 << 20  # a few kilobytes on disk
    with pytest.raises(ChunkingError, match="unpacked"):
        office_text(bomb, ".docx", BIG)


def test_an_entity_bomb_is_refused_or_bounded():
    xml = (
        '<!DOCTYPE d [<!ENTITY a "aaaaaaaaaa"><!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">'
        '<!ENTITY c "&b;&b;&b;&b;&b;&b;&b;&b;&b;&b;">]><w:p xmlns:w="w"><w:t>&c;</w:t></w:p>'
    )
    try:
        out = office_text(make_zip({"word/document.xml": xml}), ".docx", BIG)
    except ChunkingError:
        return
    assert len(out) <= BIG
