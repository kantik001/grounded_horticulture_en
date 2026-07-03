"""Tests for expect_contains matching and ranking metrics in run_rag_eval."""

import importlib.util
import os


def _load_eval_module():
    """Load scripts/run_rag_eval.py as a module without installing it."""
    path = os.path.join(os.path.dirname(__file__), "..", "scripts", "run_rag_eval.py")
    spec = importlib.util.spec_from_file_location("run_rag_eval", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def test_context_contains_singular_needle_in_plural_text():
    """A singular needle matches plural text via stemming."""
    mod = _load_eval_module()
    assert mod.context_contains("clonal plum rootstocks", "rootstock")


def test_context_contains_plural_needle_in_singular_text():
    """A plural needle matches singular text via stemming."""
    mod = _load_eval_module()
    assert mod.context_contains("clonal plum rootstock", "rootstocks")


def test_context_not_contains():
    """context_contains is False for an absent term."""
    mod = _load_eval_module()
    assert not mod.context_contains("apple yield", "scab")


def test_first_hit_rank_finds_earliest_relevant_fragment():
    """first_hit_rank returns the 1-based rank of the first matching fragment."""
    mod = _load_eval_module()
    case = {"expect_contains": ["scab"]}
    fragments = [
        {"content": "Rootstocks for intensive orchards."},
        {"content": "Apple scab appears as olive spots."},
        {"content": "Scab control with fungicides."},
    ]
    assert mod.first_hit_rank(case, fragments) == 2


def test_first_hit_rank_no_hit_returns_zero():
    """first_hit_rank returns 0 when no fragment matches."""
    mod = _load_eval_module()
    case = {"expect_contains": ["codling"]}
    fragments = [{"content": "Rootstocks for intensive orchards."}]
    assert mod.first_hit_rank(case, fragments) == 0


def test_first_hit_rank_uses_any_of_terms():
    """expect_contains_any terms also count as relevant hits."""
    mod = _load_eval_module()
    case = {"expect_contains": [], "expect_contains_any": ["marssonina", "blotch"]}
    fragments = [{"content": "Marssonina leaf blotch of apple."}]
    assert mod.first_hit_rank(case, fragments) == 1


def test_ranking_metrics_mrr_and_hit_rate():
    """ranking_metrics computes MRR and hit_rate@k, excluding unscored cases."""
    mod = _load_eval_module()
    results = [
        {"check": {"hit_rank": 1}},
        {"check": {"hit_rank": 2}},
        {"check": {"hit_rank": 0}},   # scored miss
        {"check": {"hit_rank": None}},  # out-of-scope: excluded
    ]
    metrics = mod.ranking_metrics(results)
    assert metrics["scored"] == 3
    # MRR = (1/1 + 1/2 + 0) / 3 = 0.5
    assert metrics["mrr"] == 0.5
    assert metrics["hit_rate@1"] == round(1 / 3, 3)
    assert metrics["hit_rate@3"] == round(2 / 3, 3)


def test_ranking_metrics_empty_when_nothing_scored():
    """ranking_metrics returns {} when no case has a hit rank."""
    mod = _load_eval_module()
    assert mod.ranking_metrics([{"check": {"hit_rank": None}}]) == {}
