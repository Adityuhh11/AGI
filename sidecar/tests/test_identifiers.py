import pytest

from omni_memory.store.identifiers import path_words, split_identifiers


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


@pytest.mark.parametrize(
    ("uri", "expected"),
    [
        ("Frontend/locationService.js", "Frontend locationService js location service"),
        ("Backend/src/utils/jwt.js", "Backend src utils jwt js"),
        ("add_countries.csv", "add_countries csv add countries"),
        ("Week 10 Prep.pdf", "Week 10 Prep pdf"),
        ("", ""),
    ],
)
def test_path_words(uri, expected):
    assert path_words(uri) == expected
