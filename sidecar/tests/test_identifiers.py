import pytest

from agi_memory.store.identifiers import split_identifiers


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("addCountries", "add countries"),
        ("add_countries", "add countries"),
        ("HTTPServer", "http server"),
        ("getHTTPResponse", "get http response"),
        ("version2Beta", "version2 beta"),
        ("fooBar bazQux", "foo bar baz qux"),
        ("addCountries addCountries", "add countries"),
        ("plain words only", ""),
        ("__init__", ""),
        ("", ""),
    ],
)
def test_split_identifiers(text, expected):
    assert split_identifiers(text) == expected
