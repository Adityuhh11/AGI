"""Runs the golden queries against a real index and reports, compares and gates on quality."""

from __future__ import annotations

import argparse
import json
import tempfile
from collections.abc import Callable, Sequence
from pathlib import Path

from threadpoolctl import threadpool_limits

from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.eval.golden import GoldenQuery, load_golden
from agi_memory.eval.metrics import Metrics, QueryResult, first_relevant_rank, summarize
from agi_memory.indexer import Indexer, SyncReport
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore

_COMPARED = ("top1", "recall3", "recall10", "mrr")
_LOWER_IS_BETTER = ("false_alarms", "missed_nones")


def build_index(
    store: FlatStore,
    embedder: Embedder,
    settings: Settings,
    data_dir: Path,
    source_id: str = "sample",
) -> SyncReport:
    """Index `data_dir` into `store` exactly as production would."""
    return Indexer(store, embedder, settings).sync(source_id, data_dir)


def evaluate(golden: Sequence[GoldenQuery], searcher: Searcher, k: int = 10) -> list[QueryResult]:
    results = []
    for item in golden:
        found = searcher.search_files(item.query, files=k, per_file=1)  # what the launcher shows
        uris = tuple(hit.uri for hit in found.files)
        forbidden = tuple(sorted({f for f in item.forbid for uri in uris if f in uri}))
        rank = first_relevant_rank(uris, item.expect) if item.answerable else None
        results.append(
            QueryResult(item.query, rank, forbidden, uris, found.confident, item.answerable)
        )
    return results


def baseline_dict(metrics: Metrics, embedder_id: str, device: str) -> dict[str, object]:
    return {
        "embedder": embedder_id,
        "device": device,
        "n": metrics.n,
        "top1": metrics.top1,
        "recall3": metrics.recall3,
        "recall10": metrics.recall10,
        "mrr": metrics.mrr,
        "false_alarms": metrics.false_alarms,
        "missed_nones": metrics.missed_nones,
    }


def compare_to_baseline(
    metrics: Metrics,
    baseline: dict[str, object],
    tolerance: float = 0.02,
    *,
    embedder_id: str | None = None,
) -> list[str]:
    """Messages for metrics that fell more than `tolerance`, a baseline that is not comparable
    (other golden-set size or embedder: re-record it), and any forbidden hit."""
    problems = []
    if baseline.get("n", metrics.n) != metrics.n:
        problems.append(f"baseline has {baseline['n']} queries but {metrics.n} ran: re-record it")
    if embedder_id is not None and baseline.get("embedder", embedder_id) != embedder_id:
        problems.append(
            f"baseline embedder {baseline['embedder']} is not {embedder_id}: re-record it"
        )
    for name in _COMPARED:
        old = float(baseline[name])  # type: ignore[arg-type]
        new = getattr(metrics, name)
        if new < old - tolerance:
            problems.append(f"{name} fell from {old:.3f} to {new:.3f}")
    for name in _LOWER_IS_BETTER:
        if name not in baseline:  # a baseline recorded before the flag existed
            continue
        old = float(baseline[name])  # type: ignore[arg-type]
        new = getattr(metrics, name)
        if new > old + tolerance:
            problems.append(f"{name} rose from {old:.3f} to {new:.3f}")
    if metrics.forbidden_hits:
        problems.append(
            f"{metrics.forbidden_hits} forbid hit(s): a file that must never be found was"
        )
    return problems


def render_report(
    label: str,
    embedder_id: str,
    today: str,
    metrics: Metrics,
    results: Sequence[QueryResult],
    sync: SyncReport,
    golden: Sequence[GoldenQuery],
) -> str:
    lines = [
        f"# Eval report: {label}",
        "",
        f"Date: {today}. Embedder: `{embedder_id}`. Golden queries: {metrics.n}. "
        f"Indexed: {sync.added} files, {sync.chunks_written} chunks "
        f"({sync.denied} denied, {len(sync.errors)} errors).",
        "",
        "| metric | value |",
        "|---|---|",
        f"| top1 | {metrics.top1:.3f} |",
        f"| recall@3 | {metrics.recall3:.3f} |",
        f"| recall@10 | {metrics.recall10:.3f} |",
        f"| MRR | {metrics.mrr:.3f} |",
        f"| forbidden hits | {metrics.forbidden_hits} |",
        f"| false alarms (answerable, not confident) | {metrics.false_alarms:.3f} |",
        f"| missed nones (unanswerable, confident) | {metrics.missed_nones:.3f} |",
        "",
        "## Per query",
        "",
        "| # | query | first relevant rank | expected | top result |",
        "|---|---|---|---|---|",
    ]
    for number, (item, result) in enumerate(zip(golden, results, strict=True), start=1):
        if item.answerable:
            rank = str(result.first_rank) if result.first_rank else "**miss**"
            if not result.confident:
                rank += " (not confident)"
        else:
            rank = "**confident**" if result.confident else "none (ok)"
        top = result.top_uris[0] if result.top_uris else "-"
        expected = ", ".join(item.expect) or "nothing"
        lines.append(f"| {number} | {item.query} | {rank} | {expected} | {top} |")
    return "\n".join(lines) + "\n"


def run(argv: Sequence[str], make_embedder: Callable[[bool], Embedder], today: str) -> int:
    parser = argparse.ArgumentParser(prog="python -m agi_memory.eval")
    parser.add_argument("--data", default="../Sample Data")
    parser.add_argument("--golden", default="eval/golden.yaml")
    parser.add_argument("--report-dir", default="../docs/eval")
    parser.add_argument("--label", default="baseline")
    parser.add_argument("--cpu", action="store_true", help="embed on the CPU in float32")
    parser.add_argument("--write-baseline", action="store_true")
    args = parser.parse_args(list(argv))

    golden = load_golden(Path(args.golden))
    embedder = make_embedder(args.cpu)
    with tempfile.TemporaryDirectory() as work, threadpool_limits(limits=1):
        settings = Settings(index_dir=Path(work))
        store = FlatStore(Path(work) / "index", model_id=embedder.model_id, dim=embedder.dim)
        try:
            sync = build_index(store, embedder, settings, Path(args.data))
            searcher = Searcher(store, embedder)
            searcher.calibrate()  # as the API does, so the confidence metrics are production's
            results = evaluate(golden, searcher)
        finally:
            store.close()
    metrics = summarize(results)

    report_dir = Path(args.report_dir)
    report_dir.mkdir(parents=True, exist_ok=True)
    report = render_report(args.label, embedder.model_id, today, metrics, results, sync, golden)
    (report_dir / f"{today}-{args.label}.md").write_text(report, encoding="utf-8")

    baseline_path = report_dir / "baseline.json"
    device = embedder.device
    if args.write_baseline:
        text = json.dumps(baseline_dict(metrics, embedder.model_id, device), indent=2)
        baseline_path.write_text(text + "\n", encoding="utf-8")
    if baseline_path.exists() and not args.write_baseline:
        baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
        if baseline.get("device", device) == device:
            problems = compare_to_baseline(metrics, baseline, embedder_id=embedder.model_id)
        else:  # bf16 on the GPU and fp32 on the CPU differ by up to one query: measure, do not gate
            print(f"baseline was recorded on {baseline['device']}, this run is {device}: not gated")
            problems = [f"{metrics.forbidden_hits} forbid hit(s)"] if metrics.forbidden_hits else []
    else:  # no baseline to regress from; a forbidden hit still fails the run
        problems = [f"{metrics.forbidden_hits} forbid hit(s)"] if metrics.forbidden_hits else []

    print(
        f"{metrics.n} queries: top1 {metrics.top1:.3f}, recall@3 {metrics.recall3:.3f}, "
        f"recall@10 {metrics.recall10:.3f}, MRR {metrics.mrr:.3f}, "
        f"forbidden {metrics.forbidden_hits}, false alarms {metrics.false_alarms:.3f}, "
        f"missed nones {metrics.missed_nones:.3f}"
    )
    for problem in problems:
        print(f"FAIL: {problem}")
    return 1 if problems else 0
