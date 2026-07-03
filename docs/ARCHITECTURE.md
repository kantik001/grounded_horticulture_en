# Architecture: universal grounded LLM platform

The **doctor_gardens_ai** repository is currently the **first production-shaped pack (agro bot)** on a shared core.  
Goal: clone the repo, swap the domain pack, and deploy an assistant for another vertical (HR, education, regulations, KSA, etc.) **without rewriting the core**.

See also: [DEPLOY.md](./DEPLOY.md), [knowledge-base/README.md](./knowledge-base/README.md).

---

## Three layers

```
┌─────────────────────────────────────────────────────────┐
│  Platform core (copy to a new project as-is)            │
│  Go orchestration · Python RAG · verify · admin · CI    │
└───────────────────────────┬─────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   Domain pack A      Domain pack B     Domain pack C
   (agro / apple)     (demo_hr)         (future client)
   data + config      data + config      data + config
```

| Layer | Folders / code | Changes when cloning? |
|-------|----------------|----------------------|
| **Core** | `server/`, `api/`, `rag/`, `migrations/`, `scripts/reindex_rag.py`, `scripts/run_rag_eval.py`, `docker-compose.yml`, `tests/`, `eval/` (mechanism) | No |
| **Domain pack** | `data/{domain_id}/`, `config/crops.json`, `prompts.json`, `few_shot.json`, `question_categories.json`, `onboarding.json`, `photo_templates.json`, `cv_class_labels.json`, `config/branding.json`, `webapp/` copy | **Yes** |
| **Optional modules** | CV (`cv/`, `cv_enabled`), Telegram Web App | As needed |

**`crop_id` in the API** is the **knowledge domain** (workspace) identifier. In new projects you can think of it as `domain_id`; renaming in code can come later with an alias.

---

## Data flow (text)

1. Client → Go `POST /message` (session + auth).
2. Go → Python `POST /rag/context` (`question`, `crop_id`).
3. Python hybrid search (Chroma + BM25 + reranker) returns fragments + few-shot from `config/`.
4. Go builds prompt → LLM → `cleanRAGAnswer` → `verifyRAGAnswer` → disclaimer.
5. Reply and metadata → Postgres; structured RAG log (`rag_log.go`).

Photo: `classifyAndRecommend` → Python CV (if `cv_enabled`) → LLM or `photo_templates.json`.

---

## Platform core (files)

| Component | Files | Role |
|-----------|-------|------|
| API / auth | `middleware.go`, `auth_telegram.go`, `routes.go` | Telegram, CORS, rate limit, routes |
| Chat | `message_handlers.go`, `session_handlers.go`, `postgres_store.go` | Sessions, history, photos |
| RAG + LLM | `rag_chat.go`, `rag_verify.go`, `llm.go` | Retrieval orchestration, guardrails |
| Quality | `rag_log.go`, `eval/`, `scripts/run_rag_eval.py` | Observability and regressions |
| Runtime config | `crops.go`, `config_reload.go`, `photo_templates.go`, `onboarding.go`, `branding.go` | JSON without rebuild |
| Admin | `admin.go` | Upload `.txt`, reindex |
| Python | `api/app.py`, `rag/*`, `cv/registry.py` | ML service |

---

## Domain pack (agro today)

| File | Contents |
|------|----------|
| `data/apple/*.txt` | RAG knowledge base |
| `config/crops.json` | `apple`, `demo_hr`, … + `cv_enabled` / `rag_enabled` |
| `config/prompts.json` | System prompts per domain |
| `config/few_shot.json` | Answer tone examples |
| `config/question_categories.json` | Keywords for `classify_question` (few-shot / rerank) |
| `config/onboarding.json` | Example question chips in UI |
| `config/photo_templates.json` | Templates without LLM |
| `config/cv_class_labels.json` | CV labels (agro only) |
| `config/branding.json` | Header, UI disclaimer |
| `webapp/` | Telegram channel (replaceable) |

**Sandbox `demo_hr`:** RAG without CV — proves the platform is not agro-specific.

---

## Development rule

Before merge: **“Is this core or domain pack?”**

- Universal logic → config / shared Go / `rag/`.
- Apple-specific copy, diseases, “gardener” wording → `data/`, `config/`, `branding.json`.

---

## Checklist: new project on the platform

1. `git clone` → new repository name.
2. Replace `config/branding.json`, `webapp/` texts (or your own frontend).
3. Clear / replace `data/*`, add customer documents.
4. Update `crops.json` (domains), `prompts.json`, `few_shot.json`, `question_categories.json`, `onboarding.json`.
5. `cv_enabled: false` if CV is not needed.
6. `python scripts/reindex_rag.py` (or admin reindex).
7. Copy `eval/rag_*_baseline.jsonl` → your questions; `python scripts/run_rag_eval.py`.
8. Deploy per [DEPLOY.md](./DEPLOY.md); pilot + feedback.

Estimate: **2–5 days** for MVP with ready documents and no CV.

---

## Platform roadmap (after agro pilot)

| Priority | Task |
|----------|------|
| Now | Eval, RAG logs, ARCHITECTURE, sandbox `demo_hr` |
| Next | `domain_id` alias in API, tenant in DB |
| Per customer | i18n (AR/EN), SSO, PDF/SharePoint ingest |
| Optional | Qdrant, multi-tenant SaaS |

The agro bot remains a **reference pack** and quality testbed, not the only product on the platform.
