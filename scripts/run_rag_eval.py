#!/usr/bin/env python3
"""
Run RAG eval suite: POST /rag/context and check metrics.

Default mode is retrieval-only (no LLM): substring pass/fail plus ranking
metrics (MRR and hit_rate@k over fragments containing expected terms).

--full additionally sends retrieved context to an OpenAI-compatible LLM
(env LLM_API_KEY / LLM_BASE_URL / LLM_MODEL, same variables as the Go server),
checks expected substrings in the answer, and runs the numeric verifier
(rag.verifier.verify_answer) — reports verify pass rate.

Speed options:
  --in-process   call retrieve_rag_context() without HTTP (best inside Docker classifier)
  --fast         skip cross-encoder rerank (~15x faster, smoke-eval)
  --workers N    parallel requests (HTTP or in-process; optimum ~2 per CPU)
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable, Dict, List

import requests

_ROOT = Path(__file__).resolve().parents[1]
EVAL_DIR = _ROOT / "eval"
RESULTS_DIR = EVAL_DIR / "results"

SUITES = {
    "apple": EVAL_DIR / "rag_apple_baseline.jsonl",
    "pear": EVAL_DIR / "rag_pear_baseline.jsonl",
    "plum": EVAL_DIR / "rag_plum_baseline.jsonl",
    "demo_hr": EVAL_DIR / "rag_demo_hr_baseline.jsonl",
}

_retrieve_rag_context: Callable[..., Dict[str, Any]] | None = None


def _ensure_project_root_on_path() -> None:
    """Add the project root to sys.path so rag.* imports work."""
    root = str(_ROOT)
    if root not in sys.path:
        sys.path.insert(0, root)


def _get_retrieve_rag_context() -> Callable[..., Dict[str, Any]]:
    """Lazily import and cache rag.retrieval.retrieve_rag_context for in-process mode."""
    global _retrieve_rag_context
    if _retrieve_rag_context is None:
        _ensure_project_root_on_path()
        from rag.retrieval import retrieve_rag_context

        _retrieve_rag_context = retrieve_rag_context
    return _retrieve_rag_context


def apply_fast_mode(enabled: bool) -> None:
    """Disable the cross-encoder reranker via env when fast mode is on."""
    if enabled:
        os.environ["RAG_RERANK_ENABLED"] = "false"


def context_contains(haystack: str, needle: str) -> bool:
    """Substring in context; for English allow a shortened stem (rootstock / rootstocks)."""
    h = haystack.lower()
    n = needle.lower()
    if n in h:
        return True
    if len(n) >= 5:
        stem = n[:-1]
        if len(stem) >= 4 and stem in h:
            return True
    if len(n) >= 4:
        stem4 = n[:4]
        if stem4 in h:
            return True
    return False


def expected_terms(case: Dict[str, Any]) -> List[str]:
    """All expected substrings of a case (expect_contains + expect_contains_any)."""
    return list(case.get("expect_contains") or []) + list(case.get("expect_contains_any") or [])


def first_hit_rank(case: Dict[str, Any], fragments: List[Dict[str, Any]]) -> int:
    """1-based rank of the first fragment containing any expected term; 0 = no hit.

    Single-relevant proxy for IR metrics: baselines have no ground-truth
    chunk ids, so a fragment counts as relevant when it contains at least
    one expected substring.
    """
    terms = expected_terms(case)
    if not terms:
        return 0
    for i, frag in enumerate(fragments, start=1):
        content = (frag.get("content") or "").lower()
        if any(context_contains(content, t) for t in terms):
            return i
    return 0


def ranking_metrics(results: List[Dict[str, Any]], ks: tuple = (1, 3, 5)) -> Dict[str, Any]:
    """MRR and hit_rate@k over cases that have expected terms (out-of-scope excluded)."""
    ranks = [
        r["check"]["hit_rank"]
        for r in results
        if r["check"].get("hit_rank") is not None
    ]
    if not ranks:
        return {}
    mrr = sum(1.0 / r for r in ranks if r > 0) / len(ranks)
    out: Dict[str, Any] = {"scored": len(ranks), "mrr": round(mrr, 3)}
    for k in ks:
        hits = sum(1 for r in ranks if 0 < r <= k)
        out[f"hit_rate@{k}"] = round(hits / len(ranks), 3)
    return out


def load_cases(path: Path) -> List[Dict[str, Any]]:
    """Load eval cases from a JSONL file, skipping blanks and # comments."""
    cases = []
    with path.open(encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            cases.append(json.loads(line))
    return cases


def fetch_context(rag_url: str, question: str, crop_id: str, timeout: int) -> Dict[str, Any]:
    """POST the question to /rag/context and return the body with http_status."""
    resp = requests.post(
        rag_url,
        json={"question": question, "crop_id": crop_id},
        headers={"Content-Type": "application/json; charset=utf-8"},
        timeout=timeout,
    )
    try:
        body = resp.json()
    except Exception:
        body = {"success": False, "error": resp.text[:500]}
    return {"http_status": resp.status_code, **body}


def fetch_context_local(question: str, crop_id: str) -> Dict[str, Any]:
    """Call retrieve_rag_context in-process and mimic the HTTP response shape."""
    payload = _get_retrieve_rag_context()(question, crop_id=crop_id)
    status = 200 if payload.get("success") else 422
    return {"http_status": status, **payload}


def check_retrieval(case: Dict[str, Any], ctx: Dict[str, Any]) -> Dict[str, Any]:
    """Score one case: expected substrings in context plus hit rank of fragments."""
    ok = ctx.get("success") is True and resp_status_ok(ctx)
    context_text = (ctx.get("context") or "").lower()
    fragments = ctx.get("fragments") or []

    if case.get("expect_out_of_scope"):
        # Allow soft RAG fail or short context
        ok = ok or (not context_text.strip()) or "no " in (ctx.get("error") or "").lower()
    else:
        if case.get("expect_context", True) and ok:
            ok = bool(context_text.strip()) or len(fragments) > 0

    missing = []
    for sub in case.get("expect_contains") or []:
        if not context_contains(context_text, sub):
            missing.append(sub)

    any_of = case.get("expect_contains_any") or []
    if any_of and not any(context_contains(context_text, sub) for sub in any_of):
        missing.append("any_of:" + "|".join(any_of))

    hit_rank = None
    if not case.get("expect_out_of_scope") and expected_terms(case):
        hit_rank = first_hit_rank(case, fragments)

    return {
        "passed": ok and not missing,
        "retrieval_ok": ctx.get("success"),
        "missing_in_context": missing,
        "fragment_count": len(fragments),
        "hit_rank": hit_rank,
    }


def resp_status_ok(ctx: Dict[str, Any]) -> bool:
    """True for 2xx responses or the expected 422 no-context case."""
    status = int(ctx.get("http_status", 0))
    if 200 <= status < 300:
        return True
    # Python /rag/context: 422 = expected "no context", not a transport failure.
    return status == 422 and ctx.get("success") is False


# ---------------------------------------------------------------------------
# --full mode: retrieved context -> LLM answer -> numeric verify
# ---------------------------------------------------------------------------

_FULL_SYSTEM_PROMPT = (
    "You are a knowledgeable agronomist. Answer strictly from the provided context. "
    "If the answer is not in the context, say: "
    '"The reference materials do not contain information on your question." '
    "Respond in English, include specific numbers and dosages from the context when present."
)


def llm_config() -> Dict[str, str]:
    """LLM settings from env (LLM_API_KEY / LLM_BASE_URL / LLM_MODEL)."""
    return {
        "api_key": os.environ.get("LLM_API_KEY", ""),
        "base_url": os.environ.get("LLM_BASE_URL", "https://openrouter.ai/api").rstrip("/"),
        "model": os.environ.get("LLM_MODEL", "google/gemini-2.5-flash-lite"),
    }


def call_llm(question: str, context: str, few_shot: str, timeout: int) -> str:
    """Ask the OpenAI-compatible LLM to answer the question from the given context."""
    cfg = llm_config()
    user_prompt = (
        f"<context>{context}</context>\n"
        f"<examples>{few_shot}</examples>\n"
        f"Question: {question}"
    )
    resp = requests.post(
        f"{cfg['base_url']}/v1/chat/completions",
        headers={
            "Authorization": f"Bearer {cfg['api_key']}",
            "Content-Type": "application/json",
        },
        json={
            "model": cfg["model"],
            "messages": [
                {"role": "system", "content": _FULL_SYSTEM_PROMPT},
                {"role": "user", "content": user_prompt},
            ],
        },
        timeout=timeout,
    )
    resp.raise_for_status()
    body = resp.json()
    return body["choices"][0]["message"]["content"]


def check_full_answer(
    case: Dict[str, Any], ctx: Dict[str, Any], timeout: int
) -> Dict[str, Any]:
    """LLM answer over retrieved fragments + numeric verify (rag.verifier)."""
    _ensure_project_root_on_path()
    from langchain_core.documents import Document

    from rag.verifier import verify_answer

    fragments = ctx.get("fragments") or []
    try:
        answer = call_llm(
            case["question"], ctx.get("context") or "", ctx.get("few_shot") or "", timeout
        )
    except Exception as e:
        return {"llm_ok": False, "llm_error": str(e)[:300]}

    docs = [Document(page_content=f.get("content") or "") for f in fragments]
    verify_pass, verify_reason = verify_answer(case["question"], answer, docs)

    missing = [
        sub for sub in case.get("expect_contains") or []
        if not context_contains(answer.lower(), sub)
    ]
    any_of = case.get("expect_contains_any") or []
    if any_of and not any(context_contains(answer.lower(), sub) for sub in any_of):
        missing.append("any_of:" + "|".join(any_of))

    return {
        "llm_ok": True,
        "answer_chars": len(answer),
        "verify_pass": verify_pass,
        "verify_reason": verify_reason,
        "missing_in_answer": missing,
    }


def eval_case(
    index: int,
    case: Dict[str, Any],
    *,
    in_process: bool,
    rag_url: str,
    timeout: int,
    full: bool = False,
) -> Dict[str, Any]:
    """Run one eval case: fetch context, check retrieval, optionally run --full LLM step."""
    q = case["question"]
    crop_id = case.get("crop_id", "apple")
    if in_process:
        ctx = fetch_context_local(q, crop_id)
    else:
        ctx = fetch_context(rag_url, q, crop_id, timeout)
    check = check_retrieval(case, ctx)
    result = {
        "index": index,
        "category": case.get("category"),
        "question": q,
        "crop_id": crop_id,
        "check": check,
        "rag_error": ctx.get("error"),
    }
    # Out-of-scope questions short-circuit before the LLM in production too.
    if full and not case.get("expect_out_of_scope") and ctx.get("success"):
        result["full"] = check_full_answer(case, ctx, timeout)
    return result


def full_metrics(results: List[Dict[str, Any]]) -> Dict[str, Any]:
    """Aggregate --full results: LLM success, verify pass rate, answer substring rate."""
    full_results = [r["full"] for r in results if r.get("full")]
    if not full_results:
        return {}
    answered = [f for f in full_results if f.get("llm_ok")]
    verify_passed = sum(1 for f in answered if f.get("verify_pass"))
    contains_ok = sum(1 for f in answered if not f.get("missing_in_answer"))
    out = {
        "llm_calls": len(full_results),
        "llm_answered": len(answered),
        "verify_pass_rate": round(verify_passed / len(answered), 3) if answered else 0.0,
        "answer_contains_rate": round(contains_ok / len(answered), 3) if answered else 0.0,
    }
    return out


def run_suite(
    suite_name: str,
    path: Path,
    rag_url: str,
    timeout: int,
    *,
    in_process: bool = False,
    workers: int = 1,
    full: bool = False,
) -> Dict[str, Any]:
    """Run all cases of a suite (optionally in parallel) and aggregate the summary."""
    cases = load_cases(path)
    if not cases:
        return {
            "suite": suite_name,
            "total": 0,
            "passed": 0,
            "pass_rate": 0.0,
            "cases": [],
        }

    workers = max(1, workers)
    results: List[Dict[str, Any] | None] = [None] * len(cases)

    if workers == 1:
        for i, case in enumerate(cases):
            results[i] = eval_case(
                i, case, in_process=in_process, rag_url=rag_url, timeout=timeout, full=full
            )
    else:
        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {
                executor.submit(
                    eval_case,
                    i,
                    case,
                    in_process=in_process,
                    rag_url=rag_url,
                    timeout=timeout,
                    full=full,
                ): i
                for i, case in enumerate(cases)
            }
            for future in as_completed(futures):
                item = future.result()
                results[item["index"]] = item

    assert all(r is not None for r in results)
    passed = sum(1 for r in results if r["check"]["passed"])
    total = len(cases)
    summary = {
        "suite": suite_name,
        "total": total,
        "passed": passed,
        "pass_rate": round(passed / total, 3) if total else 0.0,
        "ranking": ranking_metrics(results),  # type: ignore[arg-type]
        "cases": results,  # type: ignore[arg-type]
    }
    if full:
        summary["full"] = full_metrics(results)  # type: ignore[arg-type]
    return summary


def main() -> int:
    """CLI entry point: run selected suites, print results, write a JSON report."""
    parser = argparse.ArgumentParser(description="RAG eval (retrieval)")
    parser.add_argument(
        "--suite",
        choices=["apple", "pear", "plum", "demo_hr", "all"],
        default="apple",
        help="Question suite",
    )
    parser.add_argument(
        "--rag-url",
        default=os.environ.get("CLASSIFIER_RAG_URL", "http://localhost:5000/rag/context"),
    )
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument(
        "--in-process",
        action="store_true",
        help="Call retrieve_rag_context directly (recommended: docker exec classifier)",
    )
    parser.add_argument(
        "--fast",
        action="store_true",
        help="Disable reranker (RAG_RERANK_ENABLED=false); smoke-eval",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=1,
        metavar="N",
        help="Parallel requests (optimum ~2 when HTTP to classifier)",
    )
    parser.add_argument(
        "--full",
        action="store_true",
        help="Also generate LLM answers and run numeric verify (requires LLM_API_KEY)",
    )
    args = parser.parse_args()

    if args.full and not llm_config()["api_key"]:
        print("--full requires LLM_API_KEY (OpenAI-compatible API)", file=sys.stderr)
        return 2

    apply_fast_mode(args.fast)
    if args.in_process:
        _ensure_project_root_on_path()

    suites = list(SUITES.keys()) if args.suite == "all" else [args.suite]
    started = time.perf_counter()
    report = {
        "mode": "full" if args.full else "retrieval",
        "rag_url": "in-process" if args.in_process else args.rag_url,
        "in_process": args.in_process,
        "fast": args.fast,
        "workers": max(1, args.workers),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "suites": [],
    }
    if args.full:
        report["llm_model"] = llm_config()["model"]
    exit_code = 0
    for name in suites:
        path = SUITES[name]
        if not path.is_file():
            print(f"Missing file: {path}", file=sys.stderr)
            exit_code = 1
            continue
        summary = run_suite(
            name,
            path,
            args.rag_url,
            args.timeout,
            in_process=args.in_process,
            workers=args.workers,
            full=args.full,
        )
        report["suites"].append(summary)
        line = f"[{name}] {summary['passed']}/{summary['total']} passed ({summary['pass_rate']})"
        ranking = summary.get("ranking") or {}
        if ranking:
            line += f" | MRR {ranking['mrr']}, hit@3 {ranking.get('hit_rate@3')}"
        full = summary.get("full") or {}
        if full:
            line += (
                f" | verify {full['verify_pass_rate']}, "
                f"answer-contains {full['answer_contains_rate']} "
                f"({full['llm_answered']}/{full['llm_calls']} LLM)"
            )
        print(line)
        if summary["passed"] < summary["total"]:
            exit_code = 1
            for c in summary["cases"]:
                if not c["check"]["passed"]:
                    print(f"  FAIL: {c['question'][:60]}… missing={c['check']['missing_in_context']}")

    elapsed = time.perf_counter() - started
    report["elapsed_s"] = round(elapsed, 2)
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
    suffix = args.suite
    if args.full:
        suffix += "_full"
    if args.fast:
        suffix += "_fast"
    if args.in_process:
        suffix += "_local"
    out = RESULTS_DIR / f"{stamp}_{suffix}.json"
    out.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"Done in {elapsed:.1f}s — report: {out}")
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
