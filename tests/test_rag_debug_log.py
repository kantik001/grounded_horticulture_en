"""Tests for conditional RAG debug logging."""

from rag.debug_log import rag_debug_enabled


def test_rag_debug_disabled_by_default(monkeypatch):
    """Debug logging is off when RAG_DEBUG is unset."""
    monkeypatch.delenv("RAG_DEBUG", raising=False)
    assert rag_debug_enabled() is False


def test_rag_debug_enabled(monkeypatch):
    """RAG_DEBUG=true enables debug logging."""
    monkeypatch.setenv("RAG_DEBUG", "true")
    assert rag_debug_enabled() is True
