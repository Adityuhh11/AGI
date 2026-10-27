import json

import pytest

from omni_memory.embedding import FakeEmbedder
from omni_memory.eval.golden import GoldenQuery, load_golden
from omni_memory.eval.metrics import Metrics, QueryResult, first_relevant_rank, summarize
from omni_memory.eval.runner import baseline_dict, compare_to_baseline, render_report, run


def write_golden(path, text):
    path.write_text(text, encoding="utf-8")
    return path


def test_golden_file_loads_with_defaults(tmp_path):
    path = write_golden(
        tmp_path / "g.yaml",
        "- query: find jwt\n  expect: [jwt.js]\n- query: secrets\n  expect: [README]\n"
        "  forbid: ['.env']\n  note: privacy\n",
    )
    first, second = load_golden(path)
    assert first == GoldenQuery("find jwt", ("jwt.js",), (), "")
    assert second == GoldenQuery("secrets", ("README",), (".env",), "privacy")


@pytest.mark.parametrize(
    "text",
    [
        "just: a mapping\n",
        "- expect: [a]\n",
        "- query: ''\n  expect: [a]\n",
        "- query: q\n",
        "- query: q\n  expect: []\n",
        "- query: q\n  expect: [a]\n  surprise: 1\n",
        "- query: q\n  expect: [1]\n",
        "- query: q\n  expect: [a]\n- query: q\n  expect: [b]\n",
        "- query: q\n  expect: [a]\n  none: true\n",
        "- query: q\n  none: false\n",
        "- query: q\n  none: yes please\n",
    ],
)
def test_malformed_golden_files_are_rejected(tmp_path, text):
    with pytest.raises(ValueError, match="golden"):
        load_golden(write_golden(tmp_path / "g.yaml", text))


def test_a_query_nothing_answers_is_declared_with_none(tmp_path):
    (query,) = load_golden(write_golden(tmp_path / "g.yaml", "- query: lasagna\n  none: true\n"))
    assert query == GoldenQuery("lasagna", (), (), "")
    assert not query.answerable


def test_first_relevant_rank():
    uris = ["a/x.js", "b/jwt.js", "README.md"]
    assert first_relevant_rank(uris, ["jwt"]) == 2
    assert first_relevant_rank(uris, ["README", "x.js"]) == 1
    assert first_relevant_rank(uris, ["missing"]) is None
    assert first_relevant_rank([], ["a"]) is None


def test_summary_metrics():
    results = [
        QueryResult("q1", 1, (), ()),
        QueryResult("q2", 2, (), ()),
        QueryResult("q3", 5, (), ()),
        QueryResult("q4", None, (".env",), ()),
    ]
    m = summarize(results)
    assert (m.n, m.top1, m.recall3, m.recall10) == (4, 0.25, 0.5, 0.75)
    assert m.mrr == pytest.approx((1 + 1 / 2 + 1 / 5 + 0) / 4)
    assert m.forbidden_hits == 1


def test_unanswerable_queries_are_scored_on_confidence_not_rank():
    results = [
        QueryResult("hit", 1, (), (), confident=True),
        QueryResult("weak hit", 2, (), (), confident=False),  # a false alarm
        QueryResult("none ok", None, (), (), confident=False, answerable=False),
        QueryResult("none bad", None, (), (), confident=True, answerable=False),
        QueryResult("none bad too", None, (), (), confident=True, answerable=False),
    ]
    m = summarize(results)
    assert m.n == 5
    assert (m.top1, m.recall3) == (0.5, 1.0)  # over the two answerable queries only
    assert m.mrr == pytest.approx((1 + 1 / 2) / 2)
    assert m.false_alarms == 0.5
    assert m.missed_nones == pytest.approx(2 / 3)


def test_a_set_without_negatives_scores_zero_on_both_confidence_metrics():
    m = summarize([QueryResult("q", 1, (), ())])
    assert (m.false_alarms, m.missed_nones) == (0.0, 0.0)


def test_confidence_metrics_gate_when_the_baseline_has_them():
    baseline = {"embedder": "x", "n": 4, "top1": 1.0, "recall3": 1.0, "recall10": 1.0, "mrr": 1.0}
    worse = Metrics(4, 1.0, 1.0, 1.0, 1.0, 0, false_alarms=0.3, missed_nones=0.3)
    assert compare_to_baseline(worse, baseline) == []  # an old baseline has nothing to compare
    baseline.update(false_alarms=0.0, missed_nones=0.0)
    messages = compare_to_baseline(worse, baseline)
    assert [m.split()[0] for m in messages] == ["false_alarms", "missed_nones"]
    ok = Metrics(4, 1.0, 1.0, 1.0, 1.0, 0, false_alarms=0.02, missed_nones=0.0)
    assert compare_to_baseline(ok, baseline) == []


def test_comparing_to_the_baseline():
    baseline = {"top1": 0.8, "recall3": 0.9, "recall10": 1.0, "mrr": 0.85}
    same = Metrics(10, 0.8, 0.9, 1.0, 0.85, 0)
    assert compare_to_baseline(same, baseline) == []
    within_tolerance = Metrics(10, 0.79, 0.9, 1.0, 0.85, 0)
    assert compare_to_baseline(within_tolerance, baseline) == []
    worse = Metrics(10, 0.7, 0.9, 1.0, 0.85, 0)
    (message,) = compare_to_baseline(worse, baseline)
    assert "top1" in message
    leaked = Metrics(10, 0.8, 0.9, 1.0, 0.85, 2)
    assert any("forbid" in m for m in compare_to_baseline(leaked, baseline))


def test_baseline_dict_has_the_compared_keys():
    data = baseline_dict(Metrics(3, 0.5, 0.6, 0.7, 0.8, 0), "fake", "cpu")
    assert data == {
        "embedder": "fake",
        "device": "cpu",
        "n": 3,
        "top1": 0.5,
        "recall3": 0.6,
        "recall10": 0.7,
        "mrr": 0.8,
        "false_alarms": 0.0,
        "missed_nones": 0.0,
    }


@pytest.fixture
def corpus(tmp_path):
    data = tmp_path / "data"
    (data / "Backend").mkdir(parents=True)
    (data / "docs").mkdir()
    (data / "Backend" / "jwt.js").write_text("function refreshToken() { jwt refresh token }")
    (data / "Backend" / "addcountries.js").write_text("async function addCountries() { seed csv }")
    (data / "docs" / "README.md").write_text("# Project\nsetup the backend environment variables")
    (data / ".env").write_text("JWT_SECRET=hunter2")
    golden = write_golden(
        tmp_path / "golden.yaml",
        "- query: refresh the jwt token\n  expect: [jwt.js]\n"
        "- query: addCountries\n  expect: [addcountries.js]\n"
        "- query: environment variables\n  expect: [README]\n  forbid: ['.env']\n",
    )
    return data, golden


def make_fake(cpu):
    return FakeEmbedder(32)


def test_the_cli_runs_end_to_end_writes_a_report_and_a_baseline(tmp_path, corpus):
    data, golden = corpus
    reports = tmp_path / "reports"
    args = [
        "--data",
        str(data),
        "--golden",
        str(golden),
        "--report-dir",
        str(reports),
        "--write-baseline",
    ]
    assert run(args, make_fake, "2026-10-12") == 0
    report = (reports / "2026-10-12-baseline.md").read_text(encoding="utf-8")
    assert "refresh the jwt token" in report
    assert "top1" in report
    baseline = json.loads((reports / "baseline.json").read_text(encoding="utf-8"))
    assert baseline["embedder"] == "fake-hashed-bow-v1"
    assert baseline["n"] == 3
    assert baseline["top1"] == 1.0  # the fake embedder plus keyword search ranks these first


def test_the_cli_fails_when_quality_drops_below_the_baseline(tmp_path, corpus):
    data, _ = corpus
    reports = tmp_path / "reports"
    reports.mkdir()
    (reports / "baseline.json").write_text(
        json.dumps(
            {"embedder": "x", "n": 3, "top1": 1.0, "recall3": 1.0, "recall10": 1.0, "mrr": 1.0}
        ),
        encoding="utf-8",
    )
    impossible = write_golden(
        tmp_path / "bad.yaml",
        "- query: refresh the jwt token\n  expect: [jwt.js]\n"
        "- query: something that is not there\n  expect: [no-such-file]\n",
    )
    args = ["--data", str(data), "--golden", str(impossible), "--report-dir", str(reports)]
    assert run(args, make_fake, "2026-10-12") == 1


def test_the_cli_fails_when_a_forbidden_file_shows_up(tmp_path, corpus):
    data, _ = corpus
    reports = tmp_path / "reports"
    leaky = write_golden(
        tmp_path / "leaky.yaml",
        "- query: environment variables\n  expect: [README]\n  forbid: [README]\n",
    )
    args = ["--data", str(data), "--golden", str(leaky), "--report-dir", str(reports)]
    assert run(args, make_fake, "2026-10-12") == 1


def test_report_lists_every_query_and_misses(tmp_path):
    metrics = Metrics(2, 0.5, 0.5, 0.5, 0.5, 0)
    results = [
        QueryResult("found one", 1, (), ("a.js",)),
        QueryResult("lost one", None, (), ("z.js",)),
    ]
    golden = [GoldenQuery("found one", ("a",), (), ""), GoldenQuery("lost one", ("b",), (), "")]
    from omni_memory.indexer import SyncReport

    text = render_report("t", "fake", "2026-10-12", metrics, results, SyncReport(added=2), golden)
    assert "found one" in text and "lost one" in text
    assert "miss" in text.lower()


def test_a_baseline_from_another_golden_set_or_model_is_not_comparable():
    metrics = Metrics(10, 0.8, 0.9, 1.0, 0.85, 0)
    baseline = {"top1": 0.8, "recall3": 0.9, "recall10": 1.0, "mrr": 0.85, "n": 38, "embedder": "x"}
    problems = compare_to_baseline(metrics, baseline, embedder_id="y")
    assert any("38" in p for p in problems) and any("embedder" in p for p in problems)


def test_a_run_on_another_device_is_not_gated_on_quality(tmp_path, corpus):
    data, _ = corpus
    reports = tmp_path / "reports"
    reports.mkdir()
    (reports / "baseline.json").write_text(
        json.dumps({"embedder": "fake-hashed-bow-v1", "device": "cuda", "n": 1, "top1": 1.0,
                    "recall3": 1.0, "recall10": 1.0, "mrr": 1.0}),
        encoding="utf-8",
    )  # fmt: skip
    impossible = write_golden(
        tmp_path / "bad.yaml", "- query: nothing here\n  expect: [no-such-file]\n"
    )
    args = ["--data", str(data), "--golden", str(impossible), "--report-dir", str(reports)]
    assert run(args, make_fake, "2026-10-12") == 0  # device 'cpu' != 'cuda': measured, not gated
