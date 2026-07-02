"""Warm up RAG at classifier startup: indexes, e5, reranker, probe search."""

from __future__ import annotations

import os
import time


def _warmup_enabled() -> bool:
    return os.environ.get("RAG_WARMUP_ENABLED", "true").lower() not in (
        "0",
        "false",
        "no",
    )


def warmup_rag(crop_id: str = "apple") -> None:
    """Load Chroma/BM25, embeddings, reranker, and run a probe query."""
    if not _warmup_enabled():
        print("RAG warmup: disabled (RAG_WARMUP_ENABLED=false)")
        return

    from rag.crops_config import normalize_crop_id

    crop_id = normalize_crop_id(crop_id)
    query = os.environ.get("RAG_WARMUP_QUERY", "apple scab leaf symptoms")
    started = time.perf_counter()
    print(f"RAG warmup: starting (crop_id={crop_id})…")

    try:
        from rag.embeddings import get_embeddings
        from rag.reranker import _get_cross_encoder
        from rag.retrieval import retrieve_rag_context
        from rag.vector_store import load_vector_store

        load_vector_store()
        get_embeddings()
        _get_cross_encoder()
        payload = retrieve_rag_context(query, crop_id=crop_id)
        fragments = len(payload.get("fragments") or [])
        elapsed = time.perf_counter() - started
        if payload.get("success"):
            print(f"RAG warmup: ready in {elapsed:.1f}s, fragments={fragments}")
        else:
            print(
                f"RAG warmup: finished in {elapsed:.1f}s without context "
                f"({payload.get('error', 'no fragments')})"
            )
    except Exception as exc:
        elapsed = time.perf_counter() - started
        print(f"RAG warmup: error after {elapsed:.1f}s — {exc}")
