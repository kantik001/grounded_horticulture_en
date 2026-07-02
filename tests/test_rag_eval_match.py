"""Tests for expect_contains normalization in run_rag_eval."""

import importlib.util
import os


def _load_eval_module():
    path = os.path.join(os.path.dirname(__file__), "..", "scripts", "run_rag_eval.py")
    spec = importlib.util.spec_from_file_location("run_rag_eval", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def test_context_contains_stem():
    mod = _load_eval_module()
    assert mod.context_contains("clonal plum rootstocks", "rootstock")


def test_context_contains_plural():
    mod = _load_eval_module()
    assert mod.context_contains("clonal plum rootstocks", "rootstock")


def test_context_not_contains():
    mod = _load_eval_module()
    assert not mod.context_contains("apple yield", "scab")
