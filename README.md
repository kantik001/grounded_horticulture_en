# 🍏 grounded-horticulture — horticulture assistant

**Grounded RAG** for horticulture: answers grounded in scientific articles with fact checking, not LLM hallucinations. Telegram Mini App and browser chat with API key.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](server/)
[![Python](https://img.shields.io/badge/Python-3.11-3776AB?logo=python&logoColor=white)](api/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](docker-compose.yml)

## Demo

| Chat: question → RAG answer | Admin: articles and 👍/👎 |
|:---:|:---:|
| ![Chat demo](docs/assets/demo-chat.gif) | ![Admin demo](docs/assets/demo-admin.gif) |

[▶ Full chat recording (MP4)](docs/assets/demo-chat.mp4) · [▶ Full admin recording (MP4)](docs/assets/demo-admin.mp4)

---

## What it is

An assistant for gardeners and agronomists: **text** → hybrid search over articles → LLM answer with **verification** of numbers and dosages; **photo** → CV + recommendation (beta, no production weights in this repo).

| Component | Role |
|-----------|------|
| **Go** (`server/`) | Auth, Postgres sessions, RAG+LLM orchestration, verify, rate limit, `/metrics` |
| **Python** (`api/`, `rag/`) | Hybrid retrieval (Chroma + BM25 + reranker), CV `/classify` |
| **Web** (`webapp/`) | Chat, article upload admin, nginx in Docker |

**Access:** Telegram `initData` or browser `X-API-Key` (see `.env.example`).

> **Public repository:** git contains demo data only (`data/demo_hr/`, `data/apple/sample_*.txt`). Full article corpus and `.pth` weights stay local. [DATA_LICENSE.md](DATA_LICENSE.md) · [data/README.md](data/README.md)

## Quick start (Docker)

```bash
cp .env.example .env   # set LLM_API_KEY, API_KEYS, ADMIN_PASSWORD
docker compose up -d --build
```

Open **http://localhost/** (chat) and **http://localhost/admin.html** (admin).  
First classifier start: ~30 s model warmup, then answers in a few seconds.

```bash
docker compose ps          # all services healthy
make smoke                 # API smoke test (TELEGRAM_AUTH_DISABLED=true in .env)
docker compose down
```

After changing articles in `data/`: `make docker-reindex-apply`.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────────────┐
│  Telegram Web   │────▶│   Go Server      │────▶│ Python (Flask / gunicorn)   │
│  or browser     │◀────│  auth, sessions  │     │  /classify — CV (beta)      │
│  X-API-Key      │     │  /message (chat) │────▶│  /rag/context — hybrid RAG  │
└─────────────────┘     └────────┬─────────┘     └─────────────────────────────┘
                                 │
                                 ▼  LLM (OpenRouter / OpenAI-compatible)
                          ┌──────────────┐
                          │  LLM API     │
                          └──────────────┘
```

**Text:** question → Go → Python `/rag/context` → LLM + verify → streamed reply.  
**Photo:** image → CV → LLM or template from `photo_templates.json`.

## Stack

- **RAG:** `multilingual-e5-small`, Chroma, BM25 (RRF), `bge-reranker-base`, query expansion (`agro_glossary.json`)
- **CV:** MobileNetV2 + PyTorch (training: `cv/train_classifier.py`)
- **Backend:** Gin, PostgreSQL, Prometheus `/metrics`
- **Eval:** 68 questions in `eval/rag_*_baseline.jsonl`, `scripts/run_rag_eval.py`

Portfolio case study: [**docs/AGRO_CASE_STUDY_EN.md**](docs/AGRO_CASE_STUDY_EN.md)

## Project structure

```
grounded-horticulture/
├── server/           # Go: /message, /classify, RAG+LLM, sessions, admin API
├── api/ + rag/       # Python: /rag/context, /classify, Chroma, BM25, reranker
├── webapp/           # index.html, admin.html, app.js
├── config/           # crops, prompts, branding, few_shot, question_categories
├── data/             # .txt articles (public git: demo + samples)
├── eval/             # baseline JSONL for retrieval regression
└── docker-compose.yml
```

## Install without Docker

<details>
<summary>Local development (expand)</summary>

**Python** (port 5000):

```bash
pip install -r cv/requirements.txt
cp .env.example .env
python api/app.py
```

**Go** (port 8080):

```bash
cd server && go mod download && go run .
```

**Web app:** host `webapp/` on HTTPS; API on the Go server.

Variables: `TELEGRAM_BOT_TOKEN` or `API_KEYS`, `LLM_API_KEY`, `DATABASE_URL` — see `.env.example`.  
Dev without Telegram: `TELEGRAM_AUTH_DISABLED=true` (local only).

</details>

## API (summary)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/message` | Chat: text (RAG) or photo (CV) |
| `POST` | `/api/message/stream` | Same, SSE streaming |
| `POST` | `/api/session` | New session |
| `GET` | `/api/history` | Chat history |
| `POST` | `/api/feedback` | 👍/👎 on a reply |
| `POST` | `/classify` | CV without session |
| `POST` | `/rag/context` | Retrieval only (Python) |

Auth: `X-Telegram-Init-Data` or `X-API-Key`. `POST /chat` is deprecated — use `/message`.

<details>
<summary>Request examples (expand)</summary>

**POST /message** (JSON):

```json
{"session_id": "…", "text": "What are the signs of apple scab on leaves?", "crop_id": "apple"}
```

**POST /rag/context** (Python):

```json
{"question": "rootstocks for intensive orchard", "crop_id": "apple"}
```

</details>

## Documentation

| Topic | File |
|-------|------|
| Platform architecture | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Deploy and production | [docs/DEPLOY.md](docs/DEPLOY.md) |
| Backups | [docs/BACKUPS.md](docs/BACKUPS.md) |
| Code knowledge base | [docs/knowledge-base/README.md](docs/knowledge-base/README.md) |
| Portfolio case study | [docs/AGRO_CASE_STUDY_EN.md](docs/AGRO_CASE_STUDY_EN.md) |

## License

Source code — [Apache License 2.0](LICENSE).  
Texts in `data/` — [DATA_LICENSE.md](DATA_LICENSE.md).

## Contact

Questions and suggestions — [Issues](https://github.com/kantik001/grounded_horticulture_en/issues).
