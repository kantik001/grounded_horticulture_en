"""Unit-тесты верификатора RAG-ответов (без LLM и Chroma)."""

from langchain_core.documents import Document

from rag.verifier import extract_numbers, verify_answer


def test_extract_numbers_decimal_comma():
    assert extract_numbers("304,7 кг") == [304.7]


def test_verify_passes_with_source_and_matching_number():
    fragments = [Document(page_content="Среднее 77.", metadata={"filename": "Таблица"})]
    answer = 'Среднее 77.\n\nИсточник: "Таблица"'
    ok, reason = verify_answer("q", answer, fragments)
    assert ok, reason


def test_verify_fails_on_hallucinated_number():
    fragments = [Document(page_content="Без цифр.", metadata={"filename": "Статья"})]
    answer = 'Рентабельность 72%.\n\nИсточник: "Статья"'
    ok, reason = verify_answer("q", answer, fragments)
    assert not ok
    assert "72" in reason or "не найдены" in reason


def test_verify_fails_without_source():
    fragments = [Document(page_content="Текст.", metadata={})]
    ok, _ = verify_answer("q", "Только ответ.", fragments)
    assert not ok
