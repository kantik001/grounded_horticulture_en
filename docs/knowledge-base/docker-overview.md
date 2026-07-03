# Walkthrough: Docker and local run

**Files:** `docker-compose.yml`, `Dockerfile.server`, `Dockerfile.classifier`, `Dockerfile.webapp`, `.env`  
**Related:** [server-overview.md](./server-overview.md), [webapp-overview.md](./webapp-overview.md)

---

## Why four services

```mermaid
flowchart LR
    subgraph host [Ports on PC]
        P80[":80 → webapp:8080"]
        P8080["127.0.0.1:8080 server"]
        P5000["127.0.0.1:5000 classifier"]
    end
    webapp[Nginx webapp unprivileged]
    server[Go server]
    classifier[Python classifier]
    db[(PostgreSQL, no host port)]

    P80 --> webapp
    webapp -->|/api/ proxy| server
    P8080 --> server
    server --> db
    server --> classifier
    P5000 --> classifier
```

All app containers run as **non-root**: server and classifier as user `app` (UID 1000), webapp as unprivileged nginx (UID 101, listens on 8080). Server and classifier ports are published on **127.0.0.1 only** — public traffic goes through the webapp nginx proxy.

| Service | Image | Role |
|---------|-------|------|
| **postgres** | `postgres:16-alpine` | chat, users, feedback, analytics |
| **classifier** | `Dockerfile.classifier` | Flask `api/` + CV `cv/` + RAG `rag/` |
| **server** | `Dockerfile.server` | API, LLM, orchestration |
| **webapp** | `Dockerfile.webapp` | HTML + nginx → server |

---

## Commands (project root)

```bash
cp .env.example .env   # fill LLM_API_KEY, ADMIN_PASSWORD, etc.
docker compose up -d --build
```

Useful:

```bash
docker compose ps
docker compose logs -f server
docker compose logs -f classifier
docker compose restart server
docker compose up -d --force-recreate server   # pick up .env
docker compose down
docker compose down -v   # delete volumes (DB, chroma, uploads!)
```

Makefile: `make up`, `make logs`, `make smoke` — see `Makefile`.  
Compose project name: **`union_ai_apple`** (`name:` in `docker-compose.yml` = `PROJECT_NAME` in Makefile).

After changing Python entrypoint (`api/app.py` instead of legacy `api_server.py`) you must:

```bash
docker compose build --no-cache classifier
docker compose up -d --force-recreate classifier server webapp
```

---

## Volumes (data across restarts)

| Volume | Where | Stores |
|--------|-------|--------|
| `postgres_data` | postgres | chat tables |
| `chroma_data` | classifier `/app/chroma_db` | RAG vector index |
| `bm25_data` | classifier `/app/bm25_db` | RAG BM25 index |
| `models` | classifier `/app/models` | `.pth` (not host `./models` folder!) |
| `uploads_data` | server `/data/uploads` | user photos |

**Bind mount (from host):**

| Host path | Container | Purpose |
|-----------|-----------|---------|
| `./data` | classifier `:ro`, server `/app/data` | `.txt` articles |
| `./config` | server and classifier `/config:ro` | JSON configs without rebuild |
| `./webapp/*.html`, `app.css`, `app.js`, `nginx.conf` | webapp `:ro` | UI without rebuild |
| `./api`, `./cv`, `./rag`, `./scripts` | classifier `:ro` | dev (Python code, scripts) |
| `./eval` | classifier `/app/eval` (rw) | eval suites + writable `results/` |

Important: `MODEL_PATH=models/apple_classifier.pth` (from `/app` in container) points to **volume `models`**, not `doctor_gardens_ai/models/` on disk. To use a local folder — change compose to `./models:/app/models`.

---

## `postgres` service

- User/db: `gardener` / `gardener`; password `${POSTGRES_PASSWORD:-gardener}` (set in `.env` for prod)
- No published host port — reachable only on the compose network
- `DATABASE_URL` in server matches compose
- Healthcheck `pg_isready` — server starts after DB

---

## `classifier` service (Python: `api/` + `cv/` + `rag/`)

- Port **5000** (published on `127.0.0.1` only), entrypoint: `gunicorn -c api/gunicorn.conf.py api.app:app`, runs as non-root (user `app`, UID 1000)
- Env: `MODEL_PATH`, `ADMIN_SECRET`, `FORCE_RAG_REINDEX`, `CROPS_CONFIG_PATH`, `HF_TOKEN`, `RAG_WARMUP_ENABLED`, `RAG_*` (hybrid/rerank), `GUNICORN_*`
- Volumes: `chroma_data` → `/app/chroma_db`, `bm25_data` → `/app/bm25_db`
- Healthcheck: long `start_period: 120s` (model load + warmup)
- Endpoints: `/health`, `/classify`, `/rag/context`, `/admin/reindex`, `/crops`

HF models (e5 embeddings + bge reranker) are **baked into the image at build** (`scripts/download_hf_models.py`, `HF_HOME=/app/hf_cache`) — no network needed at container startup. Build arg `SKIP_HF_BAKE=1` skips the bake (used on CI); then the first RAG request downloads models from HuggingFace.

---

## `server` service

- Port **8080** (published on `127.0.0.1` only), runs as non-root (user `app`, UID 1000)
- Depends on healthy `postgres` + `classifier`
- In image: `main`, `migrations/`, `config/` → `/config`; compose also mounts `./config:/config:ro` so JSON edits are visible without rebuild
- `MIGRATIONS_DIR=/migrations` — SQL on startup
- Mounts `./data` to `/app/data` for admin upload (UID 1000 matches the default host user, so the bind mount stays writable)
- `UPLOAD_DIR` on volume `uploads_data`
- Healthcheck: `curl -f http://127.0.0.1:8080/health`

Key env see [server-overview.md](./server-overview.md).

Local dev without Telegram:

```env
TELEGRAM_AUTH_DISABLED=true
```

then `docker compose up -d --force-recreate server`.

---

## `webapp` service

- Host port **80** → container **8080** (image `nginxinc/nginx-unprivileged:alpine`, non-root UID 101, no root needed for port 80) → http://localhost/
- `index.html` — chat, `admin.html` — admin
- `location /api/` → proxy `http://server:8080/`
- Healthcheck: `wget http://127.0.0.1:8080/`

User opens **localhost**, API goes through nginx (initData from browser in dev).

---

## Network between containers

DNS names in compose:

- `http://classifier:5000` — from server
- `http://server:8080` — from webapp nginx
- `postgres:5432` — from server

From host: `localhost:8080` (direct to Go), `localhost/api/` (through nginx).

---

## `.env` and compose

Compose substitutes `${VAR:-default}` from `.env` in project root:

- `LLM_API_KEY`, `TELEGRAM_BOT_TOKEN`
- `POSTGRES_PASSWORD`, `ADMIN_PASSWORD`, `ADMIN_SECRET`
- `TELEGRAM_AUTH_DISABLED`
- `FORCE_RAG_REINDEX`, `HF_TOKEN`, `SKIP_HF_BAKE`

Without `.env` some values are empty — LLM and admin will not work.

---

## Common issues

| Problem | Solution |
|---------|----------|
| classifier Restarting, `api_server.py` not found | old image; `docker compose build classifier` and recreate |
| server unhealthy | `docker compose logs server`, wait for postgres/classifier |
| classifier unhealthy 2 min | normal on first start; check logs |
| webapp unhealthy | server/classifier down (webapp waits for healthy server); `docker compose ps` |
| Changes in `config/` | volume `./config:/config` (server, classifier); Go: `kill -HUP` or `CONFIG_RELOAD_INTERVAL_SEC` |
| Articles not in RAG | file in `data/apple/`, then reindex |
| Model not loading | put `.pth` in models volume or bind `./models` |
| 401 in chat | `TELEGRAM_AUTH_DISABLED=true` + recreate server |

---

## CI vs local Docker

GitHub Actions on PR: **go-test**, **go-lint**, **python-test**, **python-lint**, **docker-build**. The docker-build job builds all three images (`SKIP_HF_BAKE=1`), then runs an end-to-end **compose smoke**: `docker compose up -d --build --wait` + `scripts/smoke.sh`. RAG eval is **not** in PR CI — manual workflow **RAG Eval**. See [github-ci.yml.md](./github-ci.yml.md), [BACKUPS.md](../BACKUPS.md).

**Metrics:** `GET http://localhost:8080/metrics` (server, no auth).

---

## Brief summary

`docker-compose.yml` — **orchestration of the whole product**: one command brings up UI, API, ML, and DB. Understanding volumes and ports explains why `.env`, articles, RAG indexes (`chroma_data`, `bm25_data`), and photos “live” in different places.
