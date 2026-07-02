"""RAG retrieval tests: question categories and chunk deduplication."""

from rag.question_categories import classify_question


def test_classify_rootstock():
    assert classify_question("Which SK rootstocks for the south?") == "rootstock"


def test_classify_disease():
    assert classify_question("How to control codling moth?") == "disease"


def test_classify_relief():
    assert classify_question("Plum terraces in KBR") == "relief"
