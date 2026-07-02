# Walkthrough: RAG and LLM on Go (`server/rag_chat.go`)

**File:** `server/rag_chat.go`  
**Python:** [rag-retrieval.md](./rag-retrieval.md), [rag-verifier.md](./rag-verifier.md)  
**Called from:** `handleChat` (deprecated), `handleTextMessage` (`message_handlers.go`)

---

## Role of this file

Go does **not** search indexes itself. Chain:

1. **`fetchRAGContext`** → Python `POST /rag/context` → context, few_shot, fragments.
2. **Prompt** assembly (`buildRAGUserPrompt` + `config/prompts.json`).
3. **`callLLMCompletion`** → OpenRouter / OpenAI-compatible.
4. **Post-processing** — `cleanRAGAnswer`, `appendRAGDisclaimer`.
5. **`verifyRAGAnswer`** — numbers only from fragments (like [rag/verifier.py](../rag/verifier.py)).

---

## `fetchRAGContext(question, cropID)`

```json
POST CLASSIFIER_RAG_URL
{ "question": "...", "crop_id": "apple" }
```

Response `pythonRAGContextResponse`:

| Field | Use |
|-------|-----|
| `success` / `error` | “no articles” etc. |
| `context` | `<context>` block in prompt |
| `few_shot` | `<examples>` block |
| `fragments` | number verification |

HTTP timeout: **120s**.

---

## LLM prompt

### Template `ragUserPromptTpl`

Sections:

- `<system>` — intro from `prompts.json` (`RAGTaskIntro`)
- `<context>` — article fragments
- `<examples>` — few-shot
- `<constraints>` — do not invent, answer language per prompt, no “probably”, **no article names**
- User question

System message separately: `prompts.RAGSystem` from `promptsForCrop(cropID)`.

### Dialog history

`answerWithRAG(q, cropID, history)`:

- `history` — prior user/assistant from DB (`HistoryForLLM`).
- In `/chat` history = `nil`.
- In `/message` — prior passed for multi-turn context.

---

## Answer post-processing

### `cleanRAGAnswer`

Removes model junk: ``, `<answer>`, intros like “Let’s look…”, “handler”.

### `appendRAGDisclaimer`

1. `stripSourceAttribution` — `Source: ...` lines
2. Adds constant:

> Reference information from the knowledge base. Does not replace an in-person agronomist visit…

User **does not** see article names (product policy).

---

## `verifyRAGAnswer`

1. Concatenate all `fragments[].content`.
2. Extract numbers from answer (without disclaimer) — regex, comma → dot.
3. Each number in answer must match a number in context (±0.01).

**Failure:** user message like “⚠️ System could not confirm answer…” + reason (number 72 not found).

**No numbers in answer** — verify OK.

Tests: `rag_chat_test.go` (mirror of Python `test_verifier.py`).

---

## `answerWithRAG` — final function

Returns `(answer, success, errMsg, ragSoftFail)`:

| Case | Behavior |
|------|----------|
| Empty question | error |
| RAG `success: false` | `ragSoftFail=true`, text from Python |
| No `LLM_API_KEY` | error about key |
| LLM error | error |
| Verify fail | answer with ⚠️, `success=true` (soft refusal) |
| OK | answer with disclaimer |

---

## `handleChat` — `POST /chat` (deprecated)

Response includes headers `Deprecation: true` and `Link: </message>; rel="successor-version"`. For new integrations use `POST /message` with `session_id`.

JSON: `{ "question", "crop_id" }`.

- No Postgres save.
- Handy for RAG+LLM debugging.
- Same response codes: 400, 503 (no key), 200 + `answer`.

---

## Relation to `handleTextMessage`

Same `answerWithRAG`, but:

- before/after — `AppendMessage` in DB;
- analytics `rag_answer`;
- client response — full `messages` array.

---

## Requirements

| Component | Env / service |
|-----------|----------------|
| RAG retrieval | classifier up, articles + reindex |
| LLM | `LLM_API_KEY` |
| Prompts | `PROMPTS_CONFIG_PATH` or `config/prompts.json` |
| Crop | `normalizeCropID` + `rag_enabled` |

---

## Debugging

1. Log `RAG fetch error` — Python (Chroma/BM25/reranker).
2. “Not in reference materials” — empty RAG or LLM following prompt.
3. ⚠️ verify — number in answer not from articles (classic “72%” case).
4. 503 — no `LLM_API_KEY`.

---

## Brief summary

`rag_chat.go` — **text assistant brain on Go**: Python supplies facts, LLM formulates, Go cleans and verifies numbers. Mirrors `rag/verifier.py` ideas; Go version runs in production.
