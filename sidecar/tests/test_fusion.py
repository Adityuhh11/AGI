import pytest

from agi_memory.search.fusion import reciprocal_rank_fusion


def test_an_item_ranked_well_by_both_lists_wins():
    fused = reciprocal_rank_fusion([["a", "b", "c"], ["c", "b", "d"]])
    # c: ranks 3+1 = 1/63 + 1/61 (0.03227) just beats b: ranks 2+2 = 2/62 (0.03226); a, d once
    assert [i for i, _ in fused][:2] == ["c", "b"]


def test_scores_follow_the_formula():
    ((first, score),) = reciprocal_rank_fusion([["x"], ["x"]], k=60)
    assert first == "x"
    assert score == pytest.approx(2 / 61)


def test_ties_are_broken_by_id():
    fused = reciprocal_rank_fusion([["b"], ["a"]])
    assert [i for i, _ in fused] == ["a", "b"]  # identical scores, id order


def test_a_single_list_keeps_its_order():
    assert [i for i, _ in reciprocal_rank_fusion([["x", "y", "z"]])] == ["x", "y", "z"]


def test_empty_inputs():
    assert reciprocal_rank_fusion([]) == []
    assert reciprocal_rank_fusion([[], []]) == []


def test_a_repeated_id_counts_once_at_its_best_rank():
    fused = dict(reciprocal_rank_fusion([["a", "a", "b"]], k=60))
    assert fused["a"] == pytest.approx(1 / 61)
    assert fused["b"] == pytest.approx(1 / 63)
