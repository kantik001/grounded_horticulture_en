# Walkthrough: RAG and LLM on Go (`server/rag_chat.go`)

**Files:** `server/rag_chat.go`, `server/rag_verify.go`, `server/rag_verify_claims.go`  
**Python:** [rag-retrieval.md](./rag-retrieval.md), [rag-verifier.md](./rag-verifier.md)  
**Called from:** `handleChat` (deprecated), `handleTextMessage` (`message_handlers.go`), `handleMessageStream` (`message_stream_handlers.go`)

---

## Role of this file

Go does **not** search indexes itself. All functions take a `context.Context` (request cancellation propagates to Python and LLM calls). Chain:

1. **`fetchRAGContext(ctx, question, cropID)`** → Python `POST /rag/context` → context, few_shot, category, fragments.
2. **`buildRAGLLMMessages(ctx, q, cropID, history, sessionID)`** — retrieval + prompt assembly (`buildRAGUserPrompt` + `config/prompts.json`), returns `ragLLMInput` without calling the LLM.
3. **`callLLMCompletion`** (or `callLLMCompletionStream` for `/message/stream`) → OpenRouter / OpenAI-compatible.
4. **`finalizeRAGAnswer(ctx, raw, input, sessionID)`** — `cleanRAGAnswer`, `appendRAGDisclaimer`, then verification.
5. **Verification** — numeric check `verifyRAGAnswer` always; optional LLM claim judge `verifyRAGAnswerClaims` (see below).

`answerWithRAG` wires steps 2–4 together for the non-streaming path; `handleMessageStream` calls `buildRAGLLMMessages` and `finalizeRAGAnswer` directly around the streaming LLM call.

---

## `fetchRAGContext(ctx, question, cropID)`

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
| `category` | question category for RAG logs/analytics |
| `fragments` | answer verification |

HTTP timeout: **120s** (shared client from `http_clients.go`). HTTP 422 from Python with `success:false` is treated as an expected empty context (soft fail), not a transport error.

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

`answerWithRAG(ctx, q, cropID, history, sessionID)`:

- `history` — prior user/assistant from DB (`HistoryForLLM`).
- In `/chat` history = `nil`, sessionID = `""`.
- In `/message` and `/message/stream` — prior passed for multi-turn context; sessionID used in `[RAG]` trace logs.

---

## Answer post-processing

### `cleanRAGAnswer`

Removes model junk: ``, `<answer>`, intros like “Let’s look…”, “handler”.

### `appendRAGDisclaimer`

1. `stripSourceAttribution` — `Source: ...` lines
2. Adds constant:

> Reference information from the knowledge base. Does not replace an on-site agronomist visit…

User **does not** see article names (product policy).

---

## Verification (in `finalizeRAGAnswer`)

### 1. Numeric check — `verifyRAGAnswer` (`rag_verify.go`, always on)

1. Concatenate all `fragments[].content`.
2. Extract numbers from answer (without disclaimer) — regex, comma → dot.
3. Each number in answer must match a number in context (±0.01).

**No numbers in answer** — verify OK.

### 2. Claim-level check — `verifyRAGAnswerClaims` (`rag_verify_claims.go`, optional)

Runs **only if the numeric check passed** and `claimVerifyEnabled()`: env `RAG_VERIFY_CLAIMS_ENABLED=true` **and** `LLM_API_KEY` set.

- Sends the answer body + numbered fragments to the LLM with a strict fact-checking judge prompt.
- Judge replies with JSON `{"supported": bool, "unsupported_claims": [...]}` (parsed even if wrapped in prose or a code fence).
- An unsupported claim downgrades the answer to the same soft-fail warning as a failed numeric check.
- **Fail-open:** if the judge LLM call fails (counted in `garden_llm_errors_total`) or the verdict is unparsable, the answer is served with the numeric-only result — a judge outage never takes the chat down.

**Failure (either check):** user message like “⚠️ The system could not verify this answer against sources…” + admin reason; returned with `success=true` (soft refusal), counted as `verify_fail` + `soft_fail` in metrics.

Tests: `rag_chat_test.go` (mirror of Python `test_verifier.py`), `rag_verify_claims_test.go` (verdict parsing).

---

## `answerWithRAG` — final function

Returns `(answer, success, errMsg, ragSoftFail, trace)` — `trace` is a `RAGTrace` with latency and verify metrics for `logRAGTrace` / analytics:

| Case | Behavior |
|------|----------|
| Empty question | error |
| RAG `success: false` | `ragSoftFail=true`, text from Python |
| No `LLM_API_KEY` | error about key |
| LLM error | error (`recordLLMError`) |
| Verify fail (numbers or claims) | answer with ⚠️, `success=true` (soft refusal) |
| OK | answer with disclaimer |

---

## `handleChat` — `POST /chat` (deprecated)

Response includes headers `Deprecation: true` and `Link: </message>; rel="successor-version"`. For new integrations use `POST /message` with `session_id`.

JSON: `{ "question", "crop_id" }`.

- No Postgres save.
- Handy for RAG+LLM debugging.
- Same response codes: 400, 503 (no key), 200 + `answer`.

---

## Relation to `handleTextMessage` and `handleMessageStream`

`handleTextMessage` uses the same `answerWithRAG`, but:

- before/after — `AppendMessage` in DB;
- analytics `rag_answer` with the full trace payload;
- client response — `new_messages` (user + assistant pair).

`handleMessageStream` (`POST /message/stream`) splits the pipeline: `buildRAGLLMMessages` → `callLLMCompletionStream` with SSE `delta` events per token → `finalizeRAGAnswer` → save + `done` event.

---

## Requirements

| Component | Env / service |
|-----------|----------------|
| RAG retrieval | classifier up, articles + reindex |
| LLM | `LLM_API_KEY` |
| Prompts | `PROMPTS_CONFIG_PATH` or `config/prompts.json` |
| Crop | `normalizeCropID` + `rag_enabled` |
| Claim judge (optional) | `RAG_VERIFY_CLAIMS_ENABLED=true` + `LLM_API_KEY` |

---

## Debugging

1. Log `RAG fetch error` — Python (Chroma/BM25/reranker).
2. “Not in reference materials” — empty RAG or LLM following prompt.
3. ⚠️ verify — number in answer not from articles (classic “72%” case), or an unsupported claim flagged by the LLM judge (reason in the `[RAG]` log line).
4. Judge outage (fail-open) — answer served with numeric-only verification; a failed judge LLM call shows up only in `garden_llm_errors_total`.
5. 503 — no `LLM_API_KEY`.

---

## Brief summary

`rag_chat.go` — **text assistant brain on Go**: Python supplies facts, LLM formulates, Go cleans and verifies (numbers always, claims optionally via a fail-open LLM judge). Mirrors `rag/verifier.py` ideas; Go version runs in production.
