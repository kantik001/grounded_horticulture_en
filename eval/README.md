# RAG eval — domain quality regression

Platform-wide mechanism: one format for agro, `demo_hr`, and future clients.

## Files

| File | Domain (`crop_id`) | Questions |
|------|-------------------|-----------|
| `rag_apple_baseline.jsonl` | `apple` | 45 |
| `rag_pear_baseline.jsonl` | `pear` | 8 |
| `rag_plum_baseline.jsonl` | `plum` | 10 |
| `rag_demo_hr_baseline.jsonl` | `demo_hr` | 5 |

> Suites align with the academic article base (horticulture journal): rootstocks, slopes/terraces, nutrition, disease control (scab, codling moth). Legacy questions for removed micro-articles were dropped.

JSON line format:

```json
{
  "crop_id": "apple",
  "question": "What are signs of scab?",
  "expect_contains": ["scab", "spot"],
  "expect_context": true,
  "expect_out_of_scope": false,
  "category": "disease"
}
```

- `expect_contains` — substrings in **retrieved context** (default mode) or LLM answer (`--full`). The script allows simple English stemming (e.g. `rootstock` ↔ `rootstocks`).
- `expect_contains_any` — any one substring from the list (synonyms: `Marssonina` / `marssonina`).
- `expect_out_of_scope: true` — off-topic question; expect empty/weak context or “not in materials” in full mode.

## Run

```bash
# Retrieval-only (Python POST /rag/context) — ~4 min for all 68 questions
python scripts/run_rag_eval.py --suite apple
python scripts/run_rag_eval.py --suite all

# Fast smoke (~20 s): in-process + no rerank (inside Docker classifier)
docker compose -p union_ai_apple exec classifier \
  python scripts/run_rag_eval.py --suite all --in-process --fast

# Moderate HTTP speedup (~2×)
python scripts/run_rag_eval.py --suite all --workers 2

make eval-retrieval
```

| Flag | Effect |
|------|--------|
| `--in-process` | No HTTP; needs `chroma_db` access (Docker classifier or local) |
| `--fast` | `RAG_RERANK_ENABLED=false` — ~15× faster, 68/68 on current set |
| `--workers N` | Parallel requests; optimum **2** for one classifier |

Requires `CLASSIFIER_RAG_URL` (default `http://localhost:5000/rag/context`), except `--in-process`.

Results: `eval/results/<timestamp>_<suite>.json`.

Portfolio: [AGRO_CASE_STUDY_EN.md](../docs/AGRO_CASE_STUDY_EN.md) (EN), [AGRO_CASE_STUDY_RU.md](../docs/AGRO_CASE_STUDY_RU.md) (RU).

**GitHub CI:** unit tests on every PR; full eval via Actions → **RAG Eval** (`workflow_dispatch`). See [github-ci.yml.md](../docs/knowledge-base/github-ci.yml.md).

## When to run

- After `reindex_rag.py` / admin reindex (Chroma **and** BM25). In Docker: `make docker-reindex-apply` or reindex + `docker compose restart classifier` (see [data-pipeline.md](../docs/knowledge-base/data-pipeline.md)).
- After changes to `data/`, `prompts.json`, `few_shot.json`.
- Before pilot, **employment demo**, and release.

See [../docs/knowledge-base/quality-eval-and-rag-logs.md](../docs/knowledge-base/quality-eval-and-rag-logs.md).
