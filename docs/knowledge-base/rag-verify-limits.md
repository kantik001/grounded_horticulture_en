# RAG answer verification limits

Verification (`server/rag_verify.go`, `rag/verifier.py`) is a **heuristic against numeric hallucinations**, not full fact checking.

Contract test cases: `tests/fixtures/rag_verify_contract.json` (run in Go and Python).

---

## What is checked

1. **All numbers** are extracted from the LLM answer (regex `\b\d+(?:\.\d+)?\b`, comma → dot).
2. Before check, `Source:` lines and disclaimer text are removed.
3. Each number in the answer must **appear at least once** in a retrieved fragment within **±0.01** tolerance.
4. If the answer has **no numbers** — verification passes.

---

## What is **not** checked

| Limitation | Consequence |
|------------|-------------|
| Number may be from **another sentence** in the same fragment | “72%” in answer and “72” in a table about something else — passes |
| Number may be from **another fragment** in top-k | Match anywhere in combined context is enough |
| **Text without numbers** is not compared to sources | Model can paraphrase or invent facts without numbers |
| **Units** and number context | “50” in answer and “50” in “50 varieties” — passes |
| **Synonyms and paraphrase** | “scab” vs “Venturia inaequalis” — not compared |
| **User question** | `question` parameter in Python is unused |
| **Empty answer** | Go: fail (“Answer missing”); Python: pass if no numbers — **implementation mismatch** |

---

## Behavior on failure

Go (`finalizeRAGAnswer`): user sees a warning instead of raw LLM answer with invented number.

---

## Go ↔ Python sync

- Disclaimer: constant `ragAnswerDisclaimer` (Go) = `RAG_ANSWER_DISCLAIMER` (Python).
- On logic change — update both files **and** `tests/fixtures/rag_verify_contract.json`.
- Regressions: `go test -run Contract` in `server/`, `pytest tests/test_verify_contract.py`.

---

## Related documents

- [rag-verifier.md](./rag-verifier.md) — API walkthrough
- [ARCHITECTURE.md](../ARCHITECTURE.md) — core vs domain pack
