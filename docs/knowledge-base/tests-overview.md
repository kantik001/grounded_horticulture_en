# Walkthrough: `tests/` (Python) and project tests

**Folder:** `tests/` — **Python only** (pytest)  
**Related Go tests:** `server/*_test.go` (separate folder, same “checks” purpose)  
**CI:** `python-test` and `go-test` jobs in [github-ci.yml.md](./github-ci.yml.md)

---

## Why tests exist in this project

They verify **logic without Docker, without LLM, without Chroma, without Telegram**:

- whether numbers in a RAG answer are parsed correctly;
- whether `config/crops.json` is read correctly;
- RRF, BM25, tokenization, question categories, query expansion.

These are **unit tests** — fast, cheap, run on every push in CI.

They do not replace [smoke](scripts-overview.md), [eval](../../eval/README.md), or manual chat in the webapp.

---

## Files in `tests/`

| File | Purpose |
|------|---------|
| `conftest.py` | Shared pytest setup: project root on `PYTHONPATH` |
| `test_verifier.py` | Tests for `rag/verifier.py` |
| `test_crops_config.py` | Tests for `rag/crops_config.py` |
| `test_hybrid_search.py` | BM25, RRF, tokenization (`rag/hybrid.py`, `rag/bm25_store.py`) |
| `test_rag_retrieval.py` | `classify_question` categories (rootstock, disease, relief) |
| `test_question_categories.py` | `rag/question_categories.py`, override via `QUESTION_CATEGORIES_CONFIG_PATH` |
| `test_verify_contract.py` | verify contract vs `tests/fixtures/rag_verify_contract.json` (6 parametrized cases) |
| `test_query_expand.py` | query expansion via `config/agro_glossary.json` (`rag/query_expand.py`) |
| `test_rag_debug_log.py` | RAG debug logging (`rag/debug_log.py`) |
| `test_rag_eval_match.py` | stem matching for `expect_contains` in eval |
| `test_embeddings.py` | e5 `query:` / `passage:` prefixes |
| `test_vector_titles.py` | article titles from metadata |
| `fixtures/rag_verify_contract.json` | **cross-language contract**: same cases asserted by pytest and by Go `verify_contract_test.go` |
| `requirements-test.txt` | pytest + langchain-core + langchain-text-splitters + rank-bm25 (no PyTorch/Chroma) |

Folders `tests/__pycache__/` and `.pytest_cache/` are auto-generated and should not be in git.

---

## `conftest.py`

```python
_ROOT = .../doctor_gardens_ai
sys.path.insert(0, _ROOT)
```

So `from rag.verifier import ...` works when running from any directory.

**No DB fixtures** — tests are purely functional.

---

## `requirements-test.txt`

```
pytest>=8.0.0
langchain-core>=0.3.0
langchain-text-splitters>=1.1.2,<2
rank-bm25>=0.2.2,<0.3
```

Intentionally **no** `torch`, `langchain-chroma`, `flask`, `sentence-transformers` — CI and local `pytest` stay lightweight.

---

## `test_verifier.py` — what it checks

Module: [rag-verifier.md](./rag-verifier.md) (production duplicate in Go: `rag_chat.go`).

### `test_extract_numbers_comma_decimal`

- Input: `"304.7 kg"`
- Expected: `[304.7]` — decimal value with units.

### `test_verify_numbers_in_context`

- Fragment: “Mean 77.”
- Answer: “Mean 77.” + disclaimer
- `verify_answer` → **pass** (number is in context).

### `test_verify_hallucinated_number`

- Fragment: no digits
- Answer: “Profitability 72%”
- `verify_answer` → **fail**, reason mentions “72” or “not found”.

### `test_strip_source_attribution`

- Removes the line `Source: "Journal"`, keeps “Fact”.

---

## `test_hybrid_search.py`

- `tokenize` — Russian text and codes (`SK-4`, `M9`);
- `rrf_merge` — merge two ranked lists;
- BM25 on a mini corpus (3 documents — otherwise IDF is zero on 2 chunks).

---

## `test_rag_retrieval.py`

- `classify_question` (from `rag/question_categories.py`) categories: `rootstock`, `disease`, `relief`.

---

## `test_crops_config.py` — what it checks

Module: [rag-crops_config.md](./rag-crops_config.md).

### Fixture `crops_config_path` (autouse)

Before **each** test:

1. `monkeypatch.setenv("CROPS_CONFIG_PATH", .../config/crops.json)`
2. Reset `rag.crops_config._CONFIG = None` (and `_CONFIG_MTIME`) — re-read JSON.

### `test_normalize_crop_id_apple`

- `"apple"` and `" Apple "` → `"apple"`.

### `test_normalize_crop_id_unknown`

- `"banana_xyz"` → `ValueError` with text “Unknown crop”.

### `test_list_crops_has_apple`

- `default_crop == "apple"`;
- list includes `apple`, `rag_enabled is True`.

### `test_demo_hr_sandbox_domain`

- `demo_hr` has `rag_enabled is True` and `cv_enabled is False` (platform generality: RAG without CV).

---

## How to run locally

From the project **root**:

```powershell
pip install -r tests/requirements-test.txt
$env:CROPS_CONFIG_PATH = "config/crops.json"
pytest tests/ -v --tb=short
```

Or:

```bash
make test-py
```

Expected: **45 passed** (verifier, crops, hybrid, retrieval, question_categories, verify_contract, eval match, embeddings, titles, query_expand, debug_log).

---

## What is **not** covered by `tests/`

| Not tested | Why |
|------------|-----|
| Chroma / e5 embeddings end-to-end | heavy, slow |
| Cross-encoder reranker | needs HF + torch |
| `retrieval.py` + live index | eval suite instead of unit tests |
| LLM API | paid, non-deterministic |
| Flask `api/app.py` | no HTTP tests |
| PostgreSQL | no testcontainers |

Retrieval regressions: `python scripts/run_rag_eval.py --suite all` (see [quality-eval-and-rag-logs.md](./quality-eval-and-rag-logs.md)).

---

## Go tests (not in `tests/`, but same strategy)

Live in **`server/`**, run with `go test ./...` or `make test-go`.

| File | What it checks |
|------|----------------|
| `rag_chat_test.go` | numbers, verify, disclaimer, answer cleanup |
| `verify_contract_test.go` | verify contract vs `tests/fixtures/rag_verify_contract.json` (same fixture as pytest) |
| `rag_verify_claims_test.go` | LLM claim-judge verdict parsing (`rag_verify_claims.go`, `RAG_VERIFY_CLAIMS_ENABLED`) |
| `crops_test.go` | `normalizeCropID`, crop catalog |
| `admin_test.go` | `safeFilename`, admin helpers |
| `auth_telegram_test.go`, `auth_combined_test.go` | Telegram initData, API key |
| `api_keys_test.go` | `X-API-Key` header |
| `ratelimit_test.go` | rate limit, `gcStale` |
| `feedback_report_test.go` | `GET /admin/feedback` + `rag` field |
| `rag_log_test.go`, `llm_test.go` | helper logic |

---

## Brief summary

`tests/` — fast unit tests for RAG logic and config. Hybrid BM25 runs without Chroma; full retrieval — via **`scripts/run_rag_eval.py`** locally or the **RAG Eval** workflow in GitHub Actions.
