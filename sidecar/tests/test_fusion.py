import pytest

from agi_memory.search.fusion import blend
from agi_memory.types import KeywordHit, VectorHit


def v(chunk_id, score):
    return VectorHit(chunk_id, score)


def k(chunk_id, score):
    return KeywordHit(chunk_id, score)


def ids(blended):
    return [chunk_id for chunk_id, _ in blended]


def test_without_keyword_hits_the_cosine_order_is_kept():
    blended = blend([v("a", 0.9), v("b", 0.8), v("c", 0.7)], [])
    assert ids(blended) == ["a", "b", "c"]
    assert dict(blended) == pytest.approx({"a": 0.9, "b": 0.8, "c": 0.7})


def test_a_keyword_hit_settles_a_close_call_but_not_a_clear_one():
    close = blend([v("a", 0.80), v("b", 0.79)], [k("b", 10.0)])
    assert ids(close) == ["b", "a"]  # 0.79 + 0.02 beats 0.80
    clear = blend([v("a", 0.90), v("b", 0.70)], [k("b", 10.0)])
    assert ids(clear) == ["a", "b"]


def test_the_boost_is_relative_to_the_best_keyword_score():
    blended = dict(blend([v("a", 0.5), v("b", 0.5)], [k("a", 8.0), k("b", 4.0)]))
    assert blended["a"] == pytest.approx(0.5 + 0.02)
    assert blended["b"] == pytest.approx(0.5 + 0.01)


def test_a_keyword_only_chunk_takes_the_lowest_cosine_of_the_pool():
    blended = dict(blend([v("a", 0.9), v("b", 0.6)], [k("z", 5.0)]))
    assert blended["z"] == pytest.approx(0.6 + 0.02)
    assert ids(blend([v("a", 0.9), v("b", 0.6)], [k("z", 5.0)]))[0] == "a"


def test_keyword_only_results_when_there_are_no_vector_hits():
    blended = blend([], [k("x", 3.0), k("y", 1.5)])
    assert ids(blended) == ["x", "y"]
    assert dict(blended)["x"] == pytest.approx(0.02)


def test_empty_inputs():
    assert blend([], []) == []


def test_ties_are_broken_by_id():
    assert ids(blend([v("b", 0.5), v("a", 0.5)], [])) == ["a", "b"]


def test_a_repeated_id_counts_once_at_its_first_position():
    blended = dict(blend([v("a", 0.9), v("a", 0.1)], [k("a", 4.0), k("a", 1.0)]))
    assert blended["a"] == pytest.approx(0.9 + 0.02)


def test_a_non_positive_best_keyword_score_adds_nothing():
    assert dict(blend([v("a", 0.5)], [k("a", 0.0)]))["a"] == pytest.approx(0.5)
