# Walkthrough: `rag/verifier.py`

**Source file:** `rag/verifier.py`  
**Tests:** `tests/test_verifier.py`  
**In production:** main check in Go — `server/rag_chat.go` (`verifyRAGAnswer`, `appendRAGDisclaimer`)

---

## Why this file exists

Protection against **numeric hallucinations**: if the LLM wrote “72%” or “748.5 cm”, that number must **appear in retrieved fragments**.

Article names (`Source: "..."`) are **not** shown to the user — replaced by a general disclaimer (on Go; constant duplicated here for tests).

---

## Constant `RAG_ANSWER_DISCLAIMER`

Text at end of answer (Go adds in `appendRAGDisclaimer`):

> Reference information from the knowledge base. Does not replace an in-person agronomist visit; …

In `verifier.py` used in `strip_source_attribution` — so disclaimer numbers do not affect verification.

---

## `extract_numbers(text)`

- Replaces `,` with `.` (decimal comma).
- Finds numbers with regex: `\b\d+(?:\.\d+)?\b`.
- Returns list of `float`.

Examples: `72`, `748.5`, `496,0` → `496.0`.

---

## `strip_source_attribution(answer)`

1. Removes `Source: ...` lines (regex `_SOURCE_LINE_RE`).
2. Strips disclaimer text.
3. Collapses whitespace.

Needed to verify **answer body** without service footers.

---

## `verify_answer(question, answer, fragments)`

### Input

- `question` — currently **unused** in logic (reserved);
- `answer` — string from LLM;
- `fragments` — list of LangChain `Document` or compatible objects with `page_content`.

### Algorithm

1. Concatenate `page_content` of all fragments → `context_text`.
2. Clean answer → `body`.
3. Extract numbers from `body` and `context_text`.
4. For each number in answer: is it in context within **±0.01**?
5. If extra numbers → `(False, "Number(s) [...] not found in sources.")`.
6. If no numbers in answer → `(True, "Verification passed")`.

### Examples

| Answer | Context | Result |
|--------|---------|--------|
| “Scab — spots on leaves” | no digits | ✅ |
| “Profitability 496%” | 496 in article | ✅ |
| “Profitability 72%” | no 72 | ❌ |

---

## Python vs Go

| | `rag/verifier.py` | `server/rag_verify.go` (+ `rag_chat.go`) |
|--|-------------------|----------------------|
| In production | tests / possible future | **yes**, after every RAG answer |
| Number logic | same idea | `verifyRAGAnswer`, `extractNumbersFromText` |
| Disclaimer | constant for strip | `appendRAGDisclaimer` |

Keep logic **in sync** on changes: shared contract `tests/fixtures/rag_verify_contract.json`, tests `server/verify_contract_test.go` and `tests/test_verify_contract.py`.

**Heuristic limits** (what is not caught): [rag-verify-limits.md](./rag-verify-limits.md).

---

## Why answer sometimes says “Not in reference materials…” on verify fail

Go on failed verify may **not return** raw LLM answer to user (see `rag_chat.go`) — separate from verifier but related: model invented a number → verifier catches it.

---

## Tests

`tests/test_verifier.py`:

- decimal comma;
- pass with number in context;
- fail on 72 without 72 in context;
- strip `Source:`.

Run: `pytest tests/test_verifier.py` (no Chroma, no LLM).

---

## What to read next

| Topic | File |
|-------|------|
| Prompt and post-processing | `server/rag_chat.go` |
| Where fragments come from | [rag-retrieval.md](./rag-retrieval.md) |
| Why no “Source” in chat | discussion + `appendRAGDisclaimer` |

---

## Brief summary

`verifier.py` — **anti-hallucination for numbers** and answer cleanup utilities. Duplicated in Go in production; in Python — reference for pytest and logic documentation.
