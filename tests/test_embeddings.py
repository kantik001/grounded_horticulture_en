"""Tests for multilingual-e5 prefixes."""

from rag.embeddings import _passage, _query


def test_passage_prefix():
    assert _passage("text") == "passage: text"
    assert _passage("passage: already") == "passage: already"


def test_query_prefix():
    assert _query("question") == "query: question"
    assert _query("Query: exists") == "Query: exists"
