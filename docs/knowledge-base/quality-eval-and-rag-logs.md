# Plan: RAG Eval (3B) and RAG logs (3C)

**Status:** **implemented** (2026-07-01) — eval suites **68 questions**, `run_rag_eval.py`, `[RAG]` logs, feedback↔RAG in `GET /admin/feedback`, `/metrics`.  
**Related:** [server-rag_chat.md](./server-rag_chat.md), [metrics-and-alerts.md](./metrics-and-alerts.md)

---

## Why this matters

1. **Reproducible eval** — question set + run after reindex / prompt change.
2. **Observability** — why an answer is bad (chunks, verify, 👎, latency).

---

## Phase 3B — Eval suite

### Files (implemented)

| File | Questions | `crop_id` |
|------|-----------|-----------|
| `eval/rag_apple_baseline.jsonl` | **45** | apple |
| `eval/rag_pear_baseline.jsonl` | 8 | pear |
| `eval/rag_plum_baseline.jsonl` | 10 | plum |
| `eval/rag_demo_hr_baseline.jsonl` | 5 | demo_hr |
| **Total** | **68** | |

JSON line format:

```json
{
  "crop_id": "apple",
  "question": "What are scab symptoms on leaves?",
  "expect_contains": ["spot", "scab"],
  "expect_context": true,
  "expect_out_of_scope": false,
  "category": "disease"
}
```

- `expect_contains` — substrings in retrieval **context** (RU stemming).
- `expect_contains_any` — any one synonym is enough.
- `expect_out_of_scope: true` — question outside KB.

### Run metrics

| Metric | How to compute |
|--------|----------------|
| **retrieval pass** | `expect_contains` / out-of-scope in context |
| **verify pass rate** | share of answers without ⚠️ verify (`--full` mode via Go) |
| **manual score 1–5** | sample 10 answers (recommended before demo/pilot) |

### When to run

- after **reindex** with new articles;
- after changing **`LLM_MODEL`** or `prompts.json`;
- before **pilot**, **employer demo**, merge of large RAG PRs.

### Run

```bash
# Locally (classifier on :5000)
python scripts/run_rag_eval.py --suite all --timeout 300

# Or in-process (no HTTP)
python scripts/run_rag_eval.py --suite apple --in-process --fast

# CI: GitHub Actions → workflow RAG Eval (manual)
```

Reports: `eval/results/<timestamp>_<suite>.json`.  
See [eval/README.md](../../eval/README.md).

**Target threshold:** 100% retrieval pass (last recorded run — 68/68; reconfirm after KB changes).

**Planned:** automatic pass-rate threshold in CI on every PR (rejected due to time; see [github-ci.yml.md](./github-ci.yml.md)).

---

## Phase 3C — RAG logs and analytics

### Stdout (`rag_log.go`)

On each text answer: `crop_id`, `session_id`, `fragments`, `verify_pass`, `verify_reason`, `soft_fail`.  
**Not logged:** full prompt and LLM body.

### Prometheus (`server/metrics.go`)

`GET /metrics` — HTTP counters, LLM errors, RAG requests, verify pass/fail, latency sums.  
See [metrics-and-alerts.md](./metrics-and-alerts.md).

### Feedback + RAG in admin

`GET /admin/feedback?rating=-1&limit=50` — for each rating field **`rag`** (if `rag_answer` exists in `analytics_events`):

- `category`, `fragments`, `verify_pass`, `retrieval_ms`, `llm_ms`

Debugging 👎: admin → negative ratings → RAG metrics per message.

### Postgres (optional future)

Separate `rag_query_log` table — not implemented; stdout + metrics + analytics_events are enough for now.

---

## “Ready for pilot / demo” checklist (quality)

- [x] PVYUR corpus (~344 apple, ~42 pear, ~108 plum), reindex Chroma+BM25
- [x] Eval **68** questions, run script
- [x] Retrieval baseline **100%** (reconfirm on live index)
- [ ] Verify pass rate known (sample `--full` or manual chat)
- [x] `[RAG]` logs + `/metrics`
- [x] Feedback ↔ RAG in admin report
- [ ] 5–10 manual questions before employer demo

---

## Brief summary

**3B Eval** — 68 baseline questions, retrieval regression. **3C** — logs, metrics, feedback linkage. Full eval in CI — manual only (**RAG Eval** workflow).
