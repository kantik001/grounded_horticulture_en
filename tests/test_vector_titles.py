"""Tests for human-readable article titles."""

import os

from rag.titles import get_pretty_title, title_from_slug


def test_title_from_slug():
    """title_from_slug turns an article filename into a readable title."""
    assert title_from_slug("article12_liberty_on_sk4.txt") == "Liberty on sk4"


def test_get_pretty_title_mapped_or_slug():
    """get_pretty_title returns a mapped title or a slug-derived fallback."""
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    sample = os.path.join(root, "data", "apple", "sample_demo_scab.txt")
    title = get_pretty_title("apple", "sample_demo_scab.txt", sample)
    assert "scab" in title.lower() or "demo" in title.lower()
