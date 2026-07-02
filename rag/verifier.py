# ----------------------------------------------------------------------
# RAG answer verification: check numbers against context fragments.
# Article titles are not shown to users — only the shared disclaimer on Go.
# ----------------------------------------------------------------------
import re
from typing import List, Tuple

from langchain_core.documents import Document

RAG_ANSWER_DISCLAIMER = (
    "Reference information from the knowledge base. Does not replace an on-site agronomist visit; "
    "product decisions must follow labels and local regulations."
)

_SOURCE_LINE_RE = re.compile(r"(?im)^\s*Source:.*\n?")


def extract_numbers(text: str) -> List[float]:
    """Extract all numbers from text for comparison with context."""
    if not text:
        return []
    text = text.replace(",", ".")
    numbers = re.findall(r"\b\d+(?:\.\d+)?\b", text)
    return [float(n) for n in numbers]


def strip_source_attribution(answer: str) -> str:
    """Remove Source: lines and disclaimer before number checks."""
    s = _SOURCE_LINE_RE.sub("", answer or "")
    s = s.replace(RAG_ANSWER_DISCLAIMER, "")
    return " ".join(s.split())


def verify_answer(question: str, answer: str, fragments: List[Document]) -> Tuple[bool, str]:
    """Verify each number in the answer appears in article fragments (pytest contract)."""
    if answer is None:
        return False, "Answer is missing (None)"
    if not isinstance(answer, str):
        return False, "Answer is not a string"

    context_text = "\n".join([f.page_content for f in fragments])
    body = strip_source_attribution(answer)
    numbers_in_answer = extract_numbers(body)
    if numbers_in_answer:
        numbers_in_context = extract_numbers(context_text)
        missing_numbers = []
        for num in numbers_in_answer:
            if not any(abs(num - ctx_num) < 0.01 for ctx_num in numbers_in_context):
                missing_numbers.append(num)
        if missing_numbers:
            return False, f"Number(s) {missing_numbers} not found in sources."

    return True, "Verification passed"
