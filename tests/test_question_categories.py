"""Tests for classify_question and config/question_categories.json loading."""

import json

import pytest

import rag.question_categories as qc


@pytest.fixture(autouse=True)
def reset_qc_cache():
    """Reload the question-categories config before and after each test."""
    qc.reload_question_categories_config()
    yield
    qc.reload_question_categories_config()


def test_classify_rootstock():
    """A rootstock question is classified as 'rootstock'."""
    assert qc.classify_question("Which SK rootstocks for the south?") == "rootstock"


def test_classify_disease():
    """A pest-control question is classified as 'disease'."""
    assert qc.classify_question("How to control codling moth?") == "disease"


def test_classify_relief():
    """A terrain/terracing question is classified as 'relief'."""
    assert qc.classify_question("Plum terraces in KBR") == "relief"


def test_classify_general():
    """An unrelated question falls back to 'general'."""
    assert qc.classify_question("Hello") == "general"


def test_custom_categories_file(tmp_path, monkeypatch):
    """A custom categories file from env overrides the built-in categories."""
    cfg = {
        "default_category": "general",
        "categories": [{"id": "hr_policy", "keywords": ["vacation", "sick"]}],
    }
    p = tmp_path / "cats.json"
    p.write_text(json.dumps(cfg), encoding="utf-8")
    monkeypatch.setenv("QUESTION_CATEGORIES_CONFIG_PATH", str(p))
    qc.reload_question_categories_config()
    assert qc.classify_question("How do I request vacation?") == "hr_policy"
    assert qc.classify_question("Random question") == "general"
