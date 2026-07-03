"""BM25 hybrid tests: tokenization, RRF, BM25 search without Chroma."""

from langchain_core.documents import Document

from rag.bm25_store import build_bm25_indexes, reset_bm25_store
from rag.chunking import assign_chunk_ids
from rag.hybrid import rrf_merge, tokenize


def _make_doc(content: str, crop: str, source: str) -> Document:
    """Build a Document with the crop/source metadata BM25 expects."""
    return Document(
        page_content=content,
        metadata={"crop_id": crop, "source_file": source, "filename": source},
    )


def test_tokenize_english_and_codes():
    """tokenize splits words and keeps alphanumeric codes like SK-4 and M9."""
    tokens = tokenize("SK-4 and M9 rootstocks for plum")
    assert "sk" in tokens
    assert "4" in tokens
    assert "m9" in tokens
    assert "plum" in tokens


def test_rrf_prefers_both_lists():
    """RRF ranks an id present in both lists above single-list ids."""
    a = ["c1", "c2", "c3"]
    b = ["c2", "c4", "c1"]
    merged = rrf_merge([a, b])
    assert merged[0] == "c2"
    assert set(merged[:3]) == {"c1", "c2", "c4"}


def test_bm25_finds_exact_code():
    """BM25 finds an exact rootstock code."""
    reset_bm25_store()
    chunks = assign_chunk_ids(
        [
            _make_doc("General irrigation advice for orchards.", "plum", "article1.txt"),
            _make_doc("Liberty variety on SK-4 rootstock yields 12 t/ha.", "plum", "article2.txt"),
            # Third document needed: on a 2-chunk corpus BM25 gives zero IDF.
            _make_doc("Terracing slopes for berry plantings.", "plum", "article3.txt"),
        ]
    )
    indexes = build_bm25_indexes(chunks)
    index = indexes["plum"]
    hits = index.search("rootstock SK-4 Liberty", k=2)
    assert hits
    top_doc = index.records[hits[0]]
    assert "SK-4" in top_doc.page_content
    assert "Liberty" in top_doc.page_content


def test_bm25_empty_query():
    """BM25 search on a whitespace-only query returns no hits."""
    reset_bm25_store()
    chunks = assign_chunk_ids([_make_doc("text", "apple", "a.txt")])
    index = build_bm25_indexes(chunks)["apple"]
    assert index.search("   ", k=5) == []


def test_rerank_for_category_conditional(monkeypatch):
    """Conditional mode reranks only precision-sensitive categories."""
    from rag.hybrid import rerank_for_category

    monkeypatch.delenv("RAG_RERANK_CATEGORIES", raising=False)
    monkeypatch.setenv("RAG_RERANK_ENABLED", "true")
    monkeypatch.setenv("RAG_RERANK_CONDITIONAL", "true")
    monkeypatch.setenv("RAG_RERANK_ALWAYS", "false")

    assert rerank_for_category("rootstock") is True
    assert rerank_for_category("disease") is True
    assert rerank_for_category("general") is False
    assert rerank_for_category("irrigation") is False


def test_rerank_for_category_always(monkeypatch):
    """RAG_RERANK_ALWAYS=true enables reranking for every category."""
    from rag.hybrid import rerank_for_category

    monkeypatch.setenv("RAG_RERANK_ENABLED", "true")
    monkeypatch.setenv("RAG_RERANK_ALWAYS", "true")

    assert rerank_for_category("general") is True
