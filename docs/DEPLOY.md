# Deployment and platform cloning

Guide for the **agro bot** and for a **new project** on the same core.  
Layer architecture: [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Quick start (Docker)

```bash
cp .env.example .env
# Set LLM_API_KEY, TELEGRAM_BOT_TOKEN (or TELEGRAM_AUTH_DISABLED=true for dev)

docker compose up -d --build
```

| Service | URL |
|---------|-----|
| Web App | http://localhost/ |
| Go API | http://localhost:8080/health |
| Python | http://localhost:5000/health |

After adding articles under `data/` (rebuild Chroma **and** BM25):

```bash
make docker-reindex-apply
# or: python scripts/reindex_rag.py + restart classifier
# or POST /admin/reindex with X-Admin-Secret
```

---

## Config without rebuild

`./config` is mounted into containers as `/config` (read-only).

| Variable | File |
|----------|------|
| `CROPS_CONFIG_PATH` | `crops.json` |
| `PROMPTS_CONFIG_PATH` | `prompts.json` |
| `PHOTO_TEMPLATES_PATH` | `photo_templates.json` |
| `ONBOARDING_CONFIG_PATH` | `onboarding.json` |
| `BRANDING_CONFIG_PATH` | `branding.json` |

**Reload Go without restart:**

```bash
docker compose kill -s HUP server
```

Or set `CONFIG_RELOAD_INTERVAL_SEC=300` in `.env`.

Python `rag/crops_config.py` re-reads `crops.json` when mtime changes.

---

## Local development (without Docker)

1. Postgres + `.env` with `DATABASE_URL`.
2. `cd server && go run .`
3. Python: `cd api` or project root with `FLASK_APP=api.app` / classifier image.
4. Web: nginx or `webapp/` + `TELEGRAM_AUTH_DISABLED=true`, API on `:8080`.

---

## Eval after KB changes

```bash
# Retrieval-only (Python :5000 must be reachable)
pip install requests
set CLASSIFIER_RAG_URL=http://localhost:5000/rag/context
python scripts/run_rag_eval.py --suite apple
python scripts/run_rag_eval.py --suite demo_hr

make eval-retrieval
```

Results: `eval/results/YYYYMMDD_HHMMSS.json`.

Run after: reindex (Chroma+BM25), changes to `data/`, `prompts.json`, `few_shot.json`, `RAG_*` settings.

**GitHub:** Actions → **RAG Eval** (manual workflow) — see [knowledge-base/github-ci.yml.md](./knowledge-base/github-ci.yml.md).

**Volume backups:** [BACKUPS.md](./BACKUPS.md). **Metrics / alerts:** [knowledge-base/metrics-and-alerts.md](./knowledge-base/metrics-and-alerts.md).

---

## New customer: clone the platform

### 1. Repository

```bash
git clone <url> client-name-assistant
cd client-name-assistant
```

### 2. Domain pack

| Action | Path |
|--------|------|
| Remove or replace articles | `data/` |
| New domains | `config/crops.json` |
| Prompts and few-shot | `config/prompts.json`, `few_shot.json` |
| UI brand | `config/branding.json`, `webapp/` if needed |
| Disable CV | `"cv_enabled": false` |
| Eval questions | `eval/rag_<client>_baseline.jsonl` |

### 3. Index and verify

```bash
python scripts/reindex_rag.py
python scripts/run_rag_eval.py --suite <client>
```

### 4. Secrets and region

- `.env`: `LLM_API_KEY`, `DATABASE_URL`, CORS, Telegram or another channel.
- For KSA/GCC: hosting in-region (Bahrain/UAE), LLM in same region, PDPL — separate agreement.

### 5. Pilot

- Metrics: verify pass rate, “not in materials” rate, 👍/👎, latency.
- Do not log full LLM body (policy in ROADMAP).

---

## Smoke

```bash
make smoke
# TELEGRAM_AUTH_DISABLED=true, server on :8080
```

---

## Do not copy to a new project

- `chroma_db/` volume (created fresh).
- `postgres_data` / production sessions.
- `.env` secrets — only `.env.example` template.
