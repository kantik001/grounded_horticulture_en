"""Unit tests for the RAG answer verifier (no LLM or Chroma)."""

from langchain_core.documents import Document

from rag.verifier import RAG_ANSWER_DISCLAIMER, extract_numbers, strip_source_attribution, verify_answer


def test_extract_numbers_comma_decimal():
    """extract_numbers parses a decimal value with units."""
    assert extract_numbers("304.7 kg") == [304.7]


def test_verify_numbers_in_context():
    """An answer whose numbers appear in the context passes verification."""
    fragments = [Document(page_content="Mean 77.", metadata={"filename": "Table"})]
    answer = f"Mean 77.\n\n{RAG_ANSWER_DISCLAIMER}"
    ok, _ = verify_answer("q", answer, fragments)
    assert ok


def test_verify_hallucinated_number():
    """A number missing from the context fails verification with a reason."""
    fragments = [Document(page_content="No numbers.", metadata={"filename": "Article"})]
    answer = f"Profitability 72%.\n\n{RAG_ANSWER_DISCLAIMER}"
    ok, reason = verify_answer("q", answer, fragments)
    assert not ok
    assert "72" in reason or "not found" in reason


def test_strip_source_attribution():
    """strip_source_attribution removes the Source line but keeps the answer body."""
    raw = 'Fact.\n\nSource: "Journal"'
    body = strip_source_attribution(raw)
    assert "Source" not in body
    assert "Journal" not in body
    assert "Fact" in body
