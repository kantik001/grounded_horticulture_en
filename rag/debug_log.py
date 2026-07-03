"""Conditional RAG debug log (hot path). Enable with RAG_DEBUG=true."""

from __future__ import annotations

import os


def rag_debug_enabled() -> bool:
    """True when RAG_DEBUG env flag is on."""
    return os.environ.get("RAG_DEBUG", "").lower() in ("1", "true", "yes", "on")


def rag_debug(msg: str) -> None:
    """Print a debug message only when RAG_DEBUG is enabled."""
    if rag_debug_enabled():
        print(msg, flush=True)
