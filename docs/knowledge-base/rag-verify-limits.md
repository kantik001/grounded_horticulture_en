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

## Optional: claim-level verification (LLM judge)

`RAG_VERIFY_CLAIMS_ENABLED=true` (Go, requires `LLM_API_KEY`) adds a second pass
on top of the numeric check — `server/rag_verify_claims.go`:

1. The cleaned answer and the retrieved fragments go to an LLM judge with a strict
   fact-checking prompt.
2. The judge returns JSON `{"supported": bool, "unsupported_claims": [...]}`.
3. If any claim is unsupported, the answer is downgraded to the same soft-fail
   warning as a failed numeric check (`verify_pass=false`).

This closes the biggest gap above — **non-numeric hallucinations** (invented
diseases, treatments, procedures) that the regex check cannot catch.

Trade-offs:

| Property | Note |
|----------|------|
| Cost / latency | One extra LLM call per answer |
| **Fail-open** | Judge call/parse errors → keep numeric-only result, count `garden_llm_errors_total`; chat never blocks on judge outage |
| Judge reliability | LLM judge is itself imperfect; treat as a strong guardrail, not a proof |
| Streaming | Runs after tokens are streamed (like the numeric check); the final saved message reflects the verdict |

Off by default so retrieval-only deployments keep single-call latency.

---

## Go ↔ Python sync

- Disclaimer: constant `ragAnswerDisclaimer` (Go) = `RAG_ANSWER_DISCLAIMER` (Python).
- On logic change — update both files **and** `tests/fixtures/rag_verify_contract.json`.
- Regressions: `go test -run Contract` in `server/`, `pytest tests/test_verify_contract.py`.

---

## Related documents

- [rag-verifier.md](./rag-verifier.md) — API walkthrough
- [ARCHITECTURE.md](../ARCHITECTURE.md) — core vs domain pack
