"""Тесты префиксов multilingual-e5."""

from rag.embeddings import _passage, _query


def test_passage_prefix():
    assert _passage("текст") == "passage: текст"
    assert _passage("passage: уже") == "passage: уже"


def test_query_prefix():
    assert _query("вопрос") == "query: вопрос"
    assert _query("Query: есть") == "Query: есть"
