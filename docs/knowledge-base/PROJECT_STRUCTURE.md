# Project structure `doctor_gardens_ai`

Below is a map of the repository in its current state: where each file lives and what it does.

> Detailed walkthroughs of individual files: see [README.md](./README.md) in this folder.

## Project root

- `.env.example` — sample environment variables for local run and prod config.
- `.gitignore` — Git exclusions (artifacts, secrets, temp files).
- `.dockerignore` — exclusions when building Docker images.
- `README.md` — system overview, run, API, architecture.
- `PROJECT_STRUCTURE.md` — short root-level pointer to this map.
- `Makefile` — convenient dev commands (tests/run/utilities).
- `ruff.toml` — Python lint config for CI (`ruff check api rag cv scripts tests`).
- `LICENSE`, `DATA_LICENSE.md` — code license and content-rights notes for `data/`.
- `docker-compose.yml`, `Dockerfile.*` → [docker-overview.md](./docker-overview.md)

## `docs/knowledge-base/` (this folder)

- `README.md` — knowledge base table of contents.
- `PROJECT_STRUCTURE.md` — this file (project map).
- `python-api.md` — walkthrough of Python service `api/app.py`.
- `cv-apple_classifier.md` — walkthrough of CV model `cv/apple_classifier.py`.
- `cv-registry.md` — model factory `cv/registry.py`.
- `cv-train_classifier.md` — training `cv/train_classifier.py`.
- `github-ci.yml.md` — walkthrough of `.github/workflows/ci.yml`.
- `migrations-overview.md` — all SQL migrations `migrations/*.sql`.
- `rag-crops_config.md`, `rag-vector_store.md`, `rag-hybrid-search.md`, `rag-retrieval.md`, `rag-verifier.md`, `rag-verify-limits.md` — `rag/` modules.
- `scripts-overview.md` — utilities `scripts/`.
- `tests-overview.md` — pytest in `tests/`.
- `webapp-overview.md` — frontend `webapp/`.
- `server-overview.md`, … — Go backend.
- `config-overview.md`, `docker-overview.md`, `data-pipeline.md`, `quality-eval-and-rag-logs.md`, `metrics-and-alerts.md` — config, Docker, data, quality, monitoring.

## `.github/workflows`

- `ci.yml` — GitHub Actions CI: lint, tests, Docker build + compose smoke. → [github-ci.yml.md](./github-ci.yml.md)
- `rag-eval.yml` — manual **RAG Eval** workflow (`workflow_dispatch`): reindex + retrieval eval in the classifier image.

## `api/` (Python: HTTP for Go)

- `app.py` — Flask: `/classify`, `/rag/context`, `/health`, `/crops`, `/admin/reindex`. → [python-api.md](./python-api.md)
- `gunicorn.conf.py` — gunicorn config for Docker (workers/threads/timeout, RAG warmup).

## `cv/` (Python: Computer Vision)

- `apple_classifier.py` — MobileNetV2, inference. → [cv-apple_classifier.md](./cv-apple_classifier.md)
- `labels_config.py` — class labels from `cv_class_labels.json`
- `registry.py` — model factory and cache by `crop_id`. → [cv-registry.md](./cv-registry.md)
- `train_classifier.py` — offline `.pth` training. → [cv-train_classifier.md](./cv-train_classifier.md)
- `requirements.txt` — Python service dependencies (CV + RAG + Flask).
- `requirements-docker.txt` — Docker variant (torch installed separately from the CPU index).

## `config/` (domain and prompt configs)

→ [config-overview.md](./config-overview.md)

- `crops.json`, `prompts.json`, `photo_templates.json`, `cv_class_labels.json`, `few_shot.json`, `onboarding.json`, `question_categories.json`, `agro_glossary.json`, `article_titles.json`, `branding.json`, `api_keys.example.json`

## `data/` (RAG knowledge base)

- `README.md` — what is (and is not) in the public repo, `.txt` format, reindex commands.
- The full journal corpus (~500 files) is **local only** (gitignored); see [DATA_LICENSE.md](../../DATA_LICENSE.md).

### `data/apple/`

- `sample_demo_scab.txt`, `sample_demo_rootstock.txt` — demo articles for quick start.
- `README.txt` — note; full apple corpus is placed here locally (see [data-pipeline.md](./data-pipeline.md)).

### `data/demo_hr/` (platform sandbox)

- `policy_*.txt` — demo HR policies (vacation, sick leave, remote work, conduct); domain `demo_hr` in `crops.json` (`ui_hidden`, RAG without CV).
- Eval: [eval/rag_demo_hr_baseline.jsonl](../../eval/rag_demo_hr_baseline.jsonl).

### `data/pear/`

- `README.txt` — note/stub for pear content.

### `data/plum/`

- `README.txt` — note/stub for plum content.

## `docs/` (guides and planning)

- `ARCHITECTURE.md` — **platform core vs domain pack**, cloning checklist.
- `DEPLOY.md` — deployment, reindex, eval, new customer.
- `BACKUPS.md` — backup of Postgres and RAG volumes.
- `AGRO_CASE_STUDY_EN.md` — portfolio case study.
- `assets/` — demo GIF/MP4 for the README.
- `knowledge-base/` — code knowledge base (this folder).

## `migrations/` (DB SQL migrations)

→ Overview: [migrations-overview.md](./migrations-overview.md)

- `001_init.sql` — base schema (users, sessions, messages).
- `002_crop_id.sql` — multi-crop extension (`crop_id`).
- `003_feedback_analytics.sql` — feedback/analytics tables.

## `rag/` (Python: retrieval and verification)

- `__init__.py` — package marker.
- `crops_config.py` → [rag-crops_config.md](./rag-crops_config.md)
- `embeddings.py` — e5 `query:` / `passage:` prefixes
- `chunking.py` — shared chunking 650/80 + `chunk_id`
- `vector_store.py` → [rag-vector_store.md](./rag-vector_store.md)
- `bm25_store.py`, `hybrid.py`, `reranker.py` → [rag-hybrid-search.md](./rag-hybrid-search.md)
- `retrieval.py` → [rag-retrieval.md](./rag-retrieval.md)
- `question_categories.py` — `classify_question` keywords from `config/question_categories.json`.
- `query_expand.py` — query expansion via `config/agro_glossary.json` synonyms.
- `titles.py` — pretty article titles from `config/article_titles.json`.
- `debug_log.py` — optional RAG debug logging (`RAG_DEBUG`).
- `warmup.py` — RAG warmup on startup (`RAG_WARMUP_ENABLED`).
- `verifier.py` → [rag-verifier.md](./rag-verifier.md)
- `__pycache__/` — Python auto-cache (do not commit).

## `scripts/` (utilities)

→ [scripts-overview.md](./scripts-overview.md)

- `reindex_rag.py` — force RAG reindex (Chroma + BM25).
- `run_rag_eval.py` — run eval suites (`eval/*.jsonl`) → `eval/results/`.
- `download_hf_models.py` — pre-download HF models (e5 + reranker) during Docker build.
- `docker_build.sh` — single-image builds with a narrowed context (CI).
- `check_article_breaks.py`, `fix_article_breaks.py`, `expand_short_articles.py`, `fix_article_metadata_titles.py` — corpus cleanup/ingestion utilities for `data/`.
- `view_feedback.sql` — psql queries over `message_feedback`.
- `smoke.sh` — API smoke checks for Linux/macOS.
- `smoke.ps1` — API smoke checks for Windows PowerShell.

## `eval/` (RAG regressions)

→ [eval/README.md](../../eval/README.md)

- `rag_apple_baseline.jsonl` — **45** apple questions.
- `rag_pear_baseline.jsonl` — 8 pear questions.
- `rag_plum_baseline.jsonl` — 10 plum questions.
- `rag_demo_hr_baseline.jsonl` — 5 sandbox HR questions.
- `results/` — run reports.

## `server/` (backend API)

→ Start with [server-overview.md](./server-overview.md)

| File | Article |
|------|---------|
| `main.go`, `config.go`, `health.go`, `http_clients.go`, `metrics.go` | [server-overview.md](./server-overview.md) — startup, config, health, metrics |
| `llm.go`, `classifier_client.go`, `classify_flow.go`, `photo_recommendations.go`, `photo_templates.go`, `classify_handler.go` | [server-overview.md](./server-overview.md) — LLM and photo CV |
| `auth_telegram.go`, `auth_combined.go`, `auth_info.go`, `api_keys.go`, `rbac.go`, `middleware.go`, `ratelimit.go` | [server-auth-and-limits.md](./server-auth-and-limits.md) |
| `message_handlers.go`, `message_stream_handlers.go`, `sse.go`, `session_handlers.go`, `chat_session.go`, `postgres_store.go` | [server-chat-and-db.md](./server-chat-and-db.md) |
| `rag_verify.go`, `rag_verify_claims.go` (LLM claim judge, `RAG_VERIFY_CLAIMS_ENABLED`), `rag_log.go`, `branding.go`, `crop_guards.go`, `api_errors.go`, `routes.go`, `catalogs.go` (atomic config snapshot), `config_reload.go` | [server-overview.md](./server-overview.md) |
| `rag_chat.go` | [server-rag_chat.md](./server-rag_chat.md) |
| `admin.go`, `onboarding.go`, `feedback.go`, `feedback_report.go`, `analytics_store.go`, `crops.go` | [server-admin-and-ux-api.md](./server-admin-and-ux-api.md) |
| `go.mod`, `go.sum` | Go dependencies |
| `*_test.go` (incl. `rag_verify_claims_test.go`, `verify_contract_test.go`) | [tests-overview.md](./tests-overview.md) |

## `tests/` (Python tests)

→ [tests-overview.md](./tests-overview.md)

- `conftest.py` — pytest: `PYTHONPATH` to project root.
- `test_crops_config.py` — tests for `rag/crops_config.py`.
- `test_verifier.py` — tests for `rag/verifier.py`.
- `test_verify_contract.py` — verify contract vs `fixtures/rag_verify_contract.json` (shared with Go).
- `test_hybrid_search.py` — BM25, RRF, tokenization (no Chroma/HF).
- `test_rag_retrieval.py` — question category classification.
- `test_question_categories.py`, `test_query_expand.py`, `test_rag_debug_log.py`
- `test_rag_eval_match.py`, `test_embeddings.py`, `test_vector_titles.py`
- `fixtures/rag_verify_contract.json` — cross-language verify contract cases (pytest + `go test`).
- `requirements-test.txt` — pytest + langchain-core + langchain-text-splitters + rank-bm25 (no PyTorch/Chroma).

## `webapp/` (client UI)

→ [webapp-overview.md](./webapp-overview.md)

- `index.html`, `app.css`, `app.js` — Telegram Web App: chat, photo, onboarding, feedback.
- `admin.html` — admin: upload `.txt`, reindex (Basic auth).
- `nginx.conf` — proxy `/api/` → Go, serve HTML.

---

## Recommended code study order

1. `README.md` → quick architecture context.
2. [`ARCHITECTURE.md`](../ARCHITECTURE.md) → platform vs domain pack, cloning.
3. `docker-compose.yml` → how services connect.
4. [server-overview.md](./server-overview.md) → routes and startup.
5. [rag-vector_store.md](./rag-vector_store.md) → [rag-hybrid-search.md](./rag-hybrid-search.md) → [rag-retrieval.md](./rag-retrieval.md) → `server/rag_chat.go` → RAG core.
6. [python-api.md](./python-api.md) + [cv-apple_classifier.md](./cv-apple_classifier.md) → CV branch.
7. `migrations/*.sql` + `server/postgres_store.go` → DB and persistence.
8. `tests/`, `eval/`, `server/*_test.go` → quality and regressions.
