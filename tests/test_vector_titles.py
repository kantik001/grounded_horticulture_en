"""Tests for human-readable article titles."""

import os

from rag.titles import get_pretty_title, title_from_slug


def test_title_from_slug():
    assert title_from_slug("article12_liberty_on_sk4.txt") == "Liberty on sk4"


def test_get_pretty_title_mapped_or_slug():
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    sample = os.path.join(root, "data", "apple", "sample_demo_scab.txt")
    title = get_pretty_title("apple", "sample_demo_scab.txt", sample)
    assert "scab" in title.lower() or "demo" in title.lower()
