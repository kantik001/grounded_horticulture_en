# Project structure `doctor_gardens_ai`

Below is a map of the repository in its current state: where each file lives and what it does.

> Detailed walkthroughs of individual files: see [README.md](./README.md) in this folder.

## Project root

- `.env.example` — sample environment variables for local run and prod config.
- `.gitignore` — Git exclusions (artifacts, secrets, temp files).
- `.dockerignore` — exclusions when building Docker images.
- `README.md` — system overview, run, API, architecture.
- `Makefile` — convenient dev commands (tests/run/utilities).
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
- `rag-crops_config.md`, `rag-vector_store.md`, `rag-retrieval.md`, `rag-verifier.md` — `rag/` modules.
- `scripts-overview.md` — utilities `scripts/`.
- `tests-overview.md` — pytest in `tests/`.
- `webapp-overview.md` — frontend `webapp/`.
- `server-overview.md`, … — Go backend.
- `config-overview.md`, `docker-overview.md`, `data-pipeline.md`, `quality-eval-and-rag-logs.md` — config, Docker, data, quality plan.

## `.github/workflows`

- `ci.yml` — GitHub Actions CI: tests and build check. → [github-ci.yml.md](./github-ci.yml.md)

## `api/` (Python: HTTP for Go)

- `app.py` — Flask: `/classify`, `/rag/context`, `/health`, `/admin/reindex`. → [python-api.md](./python-api.md)

## `cv/` (Python: Computer Vision)

- `apple_classifier.py` — MobileNetV2, inference. → [cv-apple_classifier.md](./cv-apple_classifier.md)
- `labels_config.py` — class labels from `cv_class_labels.json`
- `registry.py` — model factory and cache by `crop_id`. → [cv-registry.md](./cv-registry.md)
- `train_classifier.py` — offline `.pth` training. → [cv-train_classifier.md](./cv-train_classifier.md)
- `requirements.txt` — Python service dependencies (CV + RAG + Flask).

## `config/` (domain and prompt configs)

→ [config-overview.md](./config-overview.md)

- `crops.json`, `prompts.json`, `photo_templates.json`, `cv_class_labels.json`, `few_shot.json`, `onboarding.json`, `article_titles.json`, `branding.json`

## `data/` (RAG knowledge base)

### `data/apple/` (16 files)

- `article1.txt` … `article3.txt` — source articles.
- `article4_scab.txt` … `article15_organic_calendar.txt` — diseases, care, soil, pests.
- `article16_planting_pit.txt` — planting pit (see [data-pipeline.md](./data-pipeline.md)).

### `data/demo_hr/` (platform sandbox)

- `policy_*.txt` — demo HR policies; domain `demo_hr` in `crops.json` (`ui_hidden`, RAG without CV).
- Eval: [eval/rag_demo_hr_baseline.jsonl](../../eval/rag_demo_hr_baseline.jsonl).

### `data/pear/`

- `README.txt` — note/stub for pear content.

### `data/plum/`

- `README.txt` — note/stub for plum content.

## `docs/` (guides and planning)

- `ROADMAP.md` — product development plan by phase.
- `ARCHITECTURE.md` — **platform core vs domain pack**, cloning checklist.
- `DEPLOY.md` — deployment, reindex, eval, new customer.
- `knowledge-base/` — code knowledge base (this folder).
- `LEARNING_SESSION_*.md` — removed from public branch (personal notes); see git history on `master`.

## `migrations/` (DB SQL migrations)

→ Overview: [migrations-overview.md](./migrations-overview.md)

- `001_init.sql` — base schema (users, sessions, messages).
- `002_crop_id.sql` — multi-crop extension (`crop_id`).
- `003_feedback_analytics.sql` — feedback/analytics tables.

## `rag/` (Python: retrieval and verification)

- `crops_config.py` → [rag-crops_config.md](./rag-crops_config.md)
- `embeddings.py` — e5 `query:` / `passage:` prefixes
- `chunking.py` — shared chunking 650/80 + `chunk_id`
- `vector_store.py` → [rag-vector_store.md](./rag-vector_store.md)
- `bm25_store.py`, `hybrid.py`, `reranker.py` → [rag-hybrid-search.md](./rag-hybrid-search.md)
- `retrieval.py` → [rag-retrieval.md](./rag-retrieval.md)
- `verifier.py` → [rag-verifier.md](./rag-verifier.md)
- `__pycache__/` — Python auto-cache (do not commit).

## `scripts/` (utilities)

→ [scripts-overview.md](./scripts-overview.md)

- `reindex_rag.py` — force RAG reindex.
- `run_rag_eval.py` — run eval suites (`eval/*.jsonl`) → `eval/results/`.
- `smoke.sh` — API smoke checks for Linux/macOS.
- `smoke.ps1` — API smoke checks for Windows PowerShell.

## `eval/` (RAG regressions)

→ [eval/README.md](../../eval/README.md)

- `rag_apple_baseline.jsonl` — **45** apple questions.
- `rag_pear_baseline.jsonl` — 8 pear questions.
- `rag_plum_baseline.jsonl` — 10 plum questions.
- `rag_demo_hr_baseline.jsonl` — 5 sandbox HR questions.
- `plum_miscategorized_audit.json` — internal audit (not in public branch).
- `results/` — run reports.

## `server/` (backend API)

→ Start with [server-overview.md](./server-overview.md)

| File | Article |
|------|---------|
| `main.go`, `config.go`, `health.go` | [server-overview.md](./server-overview.md) — startup, config, health |
| `llm.go`, `classifier_client.go`, `classify_flow.go`, `photo_recommendations.go`, `photo_templates.go`, `classify_handler.go` | [server-overview.md](./server-overview.md) — LLM and photo CV |
| `auth_telegram.go`, `middleware.go`, `ratelimit.go` | [server-auth-and-limits.md](./server-auth-and-limits.md) |
| `message_handlers.go`, `session_handlers.go`, `chat_session.go`, `postgres_store.go` | [server-chat-and-db.md](./server-chat-and-db.md) |
| `rag_verify.go`, `rag_log.go`, `branding.go`, `crop_guards.go`, `api_errors.go`, `routes.go`, `config_reload.go` | [server-overview.md](./server-overview.md) |
| `rag_chat.go` | [server-rag_chat.md](./server-rag_chat.md) |
| `admin.go`, `onboarding.go`, `feedback.go`, `analytics_store.go`, `crops.go` | [server-admin-and-ux-api.md](./server-admin-and-ux-api.md) |
| `go.mod`, `go.sum` | Go dependencies |
| `*_test.go` | [tests-overview.md](./tests-overview.md) |

## `tests/` (Python tests)

→ [tests-overview.md](./tests-overview.md)

- `conftest.py` — pytest: `PYTHONPATH` to project root.
- `test_crops_config.py` — tests for `rag/crops_config.py`.
- `test_verifier.py` — tests for `rag/verifier.py`.
- `test_hybrid_search.py` — BM25, RRF, tokenization (no Chroma/HF).
- `test_rag_retrieval.py` — question categories, diversify.
- `test_rag_eval_match.py`, `test_embeddings.py`, `test_vector_titles.py`
- `requirements-test.txt` — pytest + langchain-core + rank-bm25 (no PyTorch/Chroma).

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
