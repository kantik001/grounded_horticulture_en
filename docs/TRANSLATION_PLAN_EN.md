# English translation plan — phased checklist

Branch: **`public-portfolio-en`**  
Target repo: **`grounded-horticulture`** (dev.to)  
Status legend: ⬜ not started · 🔄 in progress · ✅ done

Work **one phase at a time**. After each phase: run tests + `docker compose up` + update status here.

---

## Critical decision (before phase 4)

The agro knowledge base (`data/apple/…`) is **Russian journal text**. Options:

| Strategy | UI | KB | LLM answers | Best for |
|----------|----|----|-------------|----------|
| **A. EN shell, RU RAG** | English | RU articles | Russian | Honest tech demo; weak for dev.to narrative |
| **B. EN + translated samples** | English | `sample_*.txt` in EN | English prompts | **Recommended** for public EN repo |
| **C. HR-first demo** | English | `demo_hr/*.txt` in EN | English | Platform story without agro translation cost |
| **D. Full corpus EN** | English | ~500 articles translated | English | Out of scope for portfolio |

**Recommendation:** **B + C** — English README/UI/prompts; translate `sample_*.txt` + `demo_hr/`; keep note that full agro corpus is RU-only in private repo. Agro questions still work if user asks in Russian or we add bilingual eval later.

Record chosen strategy here: ⬜ **Decision: ___** (fill when you confirm)

---

## Phase 0 — Branch and playbook ✅

- [x] Branch `public-portfolio-en` from `public-portfolio`
- [x] `docs/EN_PUBLIC_REPO.md`
- [x] `docs/TRANSLATION_PLAN_EN.md` (this file)
- [ ] Commit + push branch to `origin`

**Gate:** plan reviewed by you before phase 1.

---

## Phase 1 — Repository face (GitHub first impression)

**Goal:** English README and legal text; no code behavior change.

| File | Action | Status |
|------|--------|--------|
| `README.md` | Full EN version (mirror RU structure: demo, quick start, stack) | ⬜ |
| `DATA_LICENSE.md` | EN | ⬜ |
| `LICENSE` | Keep Apache (already EN) | ✅ |
| `data/README.md` | EN | ⬜ |
| `eval/README.md` | EN | ⬜ |
| `docs/AGRO_CASE_STUDY_EN.md` | Polish existing EN case study | ⬜ |
| `docs/PUBLIC_REPO.md` | Add EN repo URL + link to `EN_PUBLIC_REPO.md` | ⬜ |
| README cross-link | RU ↔ EN repos | ⬜ |

**Do not translate yet:** `docs/knowledge-base/*`, `docs/habr/*`, `PILOT_READINESS_AUDIT.md` (internal/pilot tone).

**Gate:** README renders on GitHub; demo GIFs work; `docker compose up` unchanged.

---

## Phase 2 — Web UI (browser + admin)

**Goal:** All visible strings in English.

| File | ~RU strings | Status |
|------|-------------|--------|
| `config/branding.json` | 8 | ⬜ |
| `config/onboarding.json` | chips | ⬜ |
| `webapp/index.html` | 14 | ⬜ |
| `webapp/admin.html` | 43 | ⬜ |
| `webapp/app.js` | 54 | ⬜ |
| `webapp/app.css` | comments only | ⬜ optional |

**Gate:** Screenshot pass — login, chat, admin, feedback tab; no Cyrillic in UI.

---

## Phase 3 — Crop names and small API surface

**Goal:** Dropdown and API responses use English names.

| Task | Files | Status |
|------|-------|--------|
| Add `name_en` to `config/crops.json` | config | ⬜ |
| Go: expose `name_en` in `/crops` (fallback `name_ru`) | `server/crops.go` | ⬜ |
| Python: error messages EN or `LOCALE=en` | `rag/retrieval.py`, `rag/crops_config.py` | ⬜ |
| `config/crops.json` `name_ru` keep for RU branch sync | — | ⬜ |

**Gate:** `GET /api/crops` returns English names; UI crop label correct.

---

## Phase 4 — RAG language (prompts, answers, demo data)

**Goal:** English answers for English demo paths.

| File | Action | Status |
|------|--------|--------|
| `config/prompts.json` | EN prompts; `Respond in English` | ⬜ |
| `config/few_shot.json` | EN examples per category | ⬜ |
| `server/rag_chat.go` | `ragUserPromptTpl` constraints in EN | ⬜ |
| `server/rag_verify.go` | `ragAnswerDisclaimer` EN | ⬜ |
| `rag/verifier.py` | disclaimer sync | ⬜ |
| `data/apple/sample_*.txt` | Translate or add `sample_*_en.txt` | ⬜ |
| `data/demo_hr/*.txt` | Translate to EN | ⬜ |
| `config/agro_glossary.json` | Keep RU terms OR add EN glossary file | ⬜ |
| Reindex | `make docker-reindex-apply` | ⬜ |
| Eval | EN questions subset or duplicate JSONL | ⬜ |

**Gate:** Ask in English on `demo_hr` or apple sample — answer in English; eval subset green.

---

## Phase 5 — Server & Python user-facing errors

**Goal:** API errors and chat messages in English (large diff — do carefully).

| Area | Files (approx.) | Status |
|------|-----------------|--------|
| Auth, rate limit | `middleware.go`, `auth_*.go` | ⬜ |
| Chat handlers | `message_*.go`, `session_handlers.go` | ⬜ |
| RAG soft failures | `rag_chat.go`, `crop_guards.go` | ⬜ |
| Admin | `admin.go` | ⬜ |
| Classify / photo | `classify_*.go`, `photo_*.go` | ⬜ |
| Flask API | `api/app.py` | ⬜ |
| Tests | Update expected strings in `*_test.go`, pytest | ⬜ |

**Suggestion:** introduce `DEFAULT_LOCALE=en` in `.env.example` and single helper `msg(key)` later — **only if** phase 5 feels too fragile. For portfolio, inline EN strings are OK.

**Gate:** `go test ./...` and `pytest` green; smoke chat errors in English.

---

## Phase 6 — Documentation (priority order)

Translate for dev.to readers, not the entire knowledge base.

| Priority | File | Status |
|----------|------|--------|
| P0 | `docs/ARCHITECTURE.md` | ⬜ |
| P0 | `docs/DEPLOY.md` | ⬜ |
| P1 | `docs/knowledge-base/README.md` | ⬜ |
| P1 | `docs/knowledge-base/server-overview.md` | ⬜ |
| P1 | `docs/knowledge-base/rag-hybrid-search.md` | ⬜ |
| P2 | Remaining `docs/knowledge-base/*.md` | ⬜ |
| P2 | `docs/ROADMAP.md`, `IMPROVEMENT_BACKLOG.md` | ⬜ optional |

**dev.to drafts:** copy structure from `docs/habr/` → `docs/devto/` (translate articles 1–7).

**Gate:** ARCHITECTURE + DEPLOY EN; at least article 1 dev.to draft ready.

---

## Phase 7 — Publish `grounded-horticulture`

- [ ] All gates from phases 1–4 passed
- [ ] `gh repo create` if not exists (public)
- [ ] Orphan push to `public-en` (see `EN_PUBLIC_REPO.md`)
- [ ] README badges + cross-link RU
- [ ] First dev.to post links to EN repo

---

## Inventory (reference)

Rough Cyrillic footprint on `public-portfolio-en` at start:

| Area | Notes |
|------|--------|
| `webapp/` | ~110 strings — phase 2 |
| `config/*.json` | branding, prompts, onboarding, few_shot, photo_templates, article_titles |
| `server/*.go` | user-facing errors, prompts, disclaimers — phase 4–5 |
| `rag/*.py` | retrieval errors, comments — phase 3–4 |
| `data/**/*.txt` | KB content — phase 4 strategy |
| `docs/**` | mostly phase 6; `AGRO_CASE_STUDY_EN` partial |
| `tests/` | assertions on RU strings — update with phase 5 |

**Already English:** `LICENSE`, most code identifiers, CI workflow names, `AGRO_CASE_STUDY_EN.md` (partial).

---

## Workflow per phase (repeat)

1. You say: «делаем phase N»
2. Agent translates only files in that phase
3. `go test ./...` + `pytest` + manual UI check
4. Commit: `en(phase-N): …`
5. Update checkboxes in this file
6. Optional: orphan push to `public-en` preview

---

## Commits on this branch (log)

| Date | Phase | Commit |
|------|-------|--------|
| 2026-07-02 | 0 | (pending) `docs: EN public repo playbook and translation plan` |
