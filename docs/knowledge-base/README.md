# Project knowledge base

Documentation for self-study: you or a colleague can open the right file and quickly understand what a module does.

**Platform (core vs domain pack):** [../ARCHITECTURE.md](../ARCHITECTURE.md), [../DEPLOY.md](../DEPLOY.md), [../../eval/README.md](../../eval/README.md).

## Contents

| Document | Description |
|----------|-------------|
| [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) | Repository map: folders and files, what each does |
| [python-api.md](./python-api.md) | Detailed walkthrough of `api/app.py` (Python Flask, not Go) |
| [cv-apple_classifier.md](./cv-apple_classifier.md) | PyTorch MobileNetV2: disease classes, inference, `.pth` weights |
| [cv-registry.md](./cv-registry.md) | Model factory and cache by `crop_id`, `MODEL_PATH`, `cv_enabled` |
| [cv-train_classifier.md](./cv-train_classifier.md) | Model training, dataset, saving `apple_classifier.pth` |
| [github-ci.yml.md](./github-ci.yml.md) | GitHub Actions: CI (5 jobs: lint, tests, docker + compose smoke) + manual **RAG Eval** |
| [migrations-overview.md](./migrations-overview.md) | SQL migrations 001–003: syntax, table relations, apply on startup |

### RAG (`rag/` package)

| Document | Description |
|----------|-------------|
| [rag-crops_config.md](./rag-crops_config.md) | `config/crops.json`, `crop_id`, `rag_enabled` / `cv_enabled` |
| [rag-vector_store.md](./rag-vector_store.md) | Chroma + BM25, chunking, embeddings, reindex |
| [rag-hybrid-search.md](./rag-hybrid-search.md) | BM25 hybrid, RRF, cross-encoder reranker, env |
| [rag-retrieval.md](./rag-retrieval.md) | Search, context, few-shot, `POST /rag/context` |
| [rag-verifier.md](./rag-verifier.md) | Answer number check, disclaimer (logic duplicated in Go) |
| [rag-verify-limits.md](./rag-verify-limits.md) | Verify heuristic limits, Go/Python differences |

**RAG reading order:** `crops_config` → `vector_store` → `hybrid-search` → `retrieval` → `verifier` → `server/rag_chat.go`

### Utilities (`scripts/`)

| Document | Description |
|----------|-------------|
| [scripts-overview.md](./scripts-overview.md) | `reindex_rag.py`, `run_rag_eval.py`, `smoke.ps1`/`smoke.sh`, corpus utilities — when to run, what they check |

### Tests

| Document | Description |
|----------|-------------|
| [tests-overview.md](./tests-overview.md) | `tests/` (pytest), coverage, run, relation to Go tests and CI |

### Web UI (`webapp/`)

| Document | Description |
|----------|-------------|
| [webapp-overview.md](./webapp-overview.md) | `index.html`, `admin.html`, `nginx.conf` — chat, admin, proxy |

### Go backend (`server/`)

| Document | Description |
|----------|-------------|
| [server-overview.md](./server-overview.md) | Startup, config, routes, `server/*.go` split (LLM, CV, health) |
| [server-auth-and-limits.md](./server-auth-and-limits.md) | Telegram initData, CORS, rate limit |
| [server-chat-and-db.md](./server-chat-and-db.md) | `POST /message`, Postgres, photos, sessions |
| [server-rag_chat.md](./server-rag_chat.md) | RAG + LLM + verify + disclaimer |
| [server-admin-and-ux-api.md](./server-admin-and-ux-api.md) | Admin, crops, onboarding, feedback |

**Server reading order:** overview → auth → chat-and-db → rag_chat (after Python RAG) → admin-and-ux

### Infrastructure, config, data, quality

| Document | Description |
|----------|-------------|
| [config-overview.md](./config-overview.md) | `config/*.json`: crops, prompts, branding, `photo_templates`, few-shot, onboarding |
| [docker-overview.md](./docker-overview.md) | docker-compose, 4 services, volumes, ports, `.env` |
| [data-pipeline.md](./data-pipeline.md) | Upload `.txt` articles, reindex, train `.pth` |
| [quality-eval-and-rag-logs.md](./quality-eval-and-rag-logs.md) | Eval suites (68 Q), `run_rag_eval.py`, `[RAG]` logs, feedback |
| [metrics-and-alerts.md](./metrics-and-alerts.md) | `GET /metrics`, PromQL, alerts, feedback+RAG |

## How to use

1. Do not know where to look → **PROJECT_STRUCTURE.md**.
2. Studying a specific file → open the matching `*.md` in this folder.
3. Pilot / demo readiness → [`../AGRO_CASE_STUDY_EN.md`](../AGRO_CASE_STUDY_EN.md), [`../../DATA_LICENSE.md`](../../DATA_LICENSE.md).

## Adding new articles

Naming: `{path-to-file-with-dashes}.md`, for example:

- `server-rag_chat.md` → `server/rag_chat.go`
- `python-api.md` → `api/app.py`
- `cv-registry.md` → `cv/registry.py`
- `rag-retrieval.md` → `rag/retrieval.py`

At the start of each article specify the **source file** in the repo and **related modules**.
