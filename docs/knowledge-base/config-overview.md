# Walkthrough: `config/` folder

**Folder:** `config/` — JSON configs without code rebuild (partially mounted in Docker).  
**Readers:** Go (`server/`), Python (`rag/`, `cv/`)

---

## Files

| File | Loaded by | Purpose |
|------|-----------|---------|
| `crops.json` | Go + Python | Crops, `cv_enabled`, `rag_enabled` |
| `prompts.json` | Go | System prompts for RAG and photo by `crop_id` |
| `photo_templates.json` | Go | Static photo recommendations if LLM unavailable |
| `cv_class_labels.json` | Python CV | Class labels by `crop_id` (order = training index) |
| `few_shot.json` | Python `retrieval.py` | Q&A examples for LLM prompt |
| `onboarding.json` | Go | “Sample question” chips in Web App |
| `question_categories.json` | Python `question_categories.py` | Keywords for `classify_question` (few-shot) |
| `agro_glossary.json` | Python `query_expand.py` | Synonyms for query expansion (optional) |
| `branding.json` | Go | Header, disclaimer, `photo_beta_notice`, UI labels |
| `article_titles.json` | Python `vector_store.py` | Display article titles in metadata (optional) |

Code details: [rag-crops_config.md](./rag-crops_config.md), [server-admin-and-ux-api.md](./server-admin-and-ux-api.md).

---

## `crops.json`

```json
{
  "default_crop": "apple",
  "crops": {
    "apple": { "name_ru": "Apple", "emoji": "🍎", "cv_enabled": true, "rag_enabled": true },
    "pear": { "name_ru": "Pear", "emoji": "🍐", "cv_enabled": false, "rag_enabled": true },
    "plum": { "name_ru": "Plum", "emoji": "🍑", "cv_enabled": false, "rag_enabled": true },
    "demo_hr": { "cv_enabled": false, "rag_enabled": true, "ui_hidden": true }
  }
}
```

- **`ui_hidden: true`** — domain exists in API/eval but **not shown** in `cropSelect` (platform sandbox).
- **UI:** `GET /api/crops` → dropdown in `index.html` (hidden domains excluded).
- **Python:** `normalize_crop_id`, `crop_id` filter in hybrid search, CV check.
- **Go:** `normalizeCropID`, `requireCVEnabled` / `requireRAGEnabled` before CV and RAG.

Adding a crop: entry in JSON + folder `data/{crop_id}/` + if needed blocks in `prompts.json`, `few_shot.json`, `onboarding.json`.

Env: `CROPS_CONFIG_PATH` (in Docker: `/config/crops.json` on server and classifier).

**Reload:** Go — `SIGHUP` or `CONFIG_RELOAD_INTERVAL_SEC`; Python `rag/crops_config.py` — on file mtime at next `load_crops_config()`.

---

## `cv_class_labels.json`

```json
{ "apple": ["healthy_apple", "apple_scab", ...] }
```

- **Python:** `cv/labels_config.py` → `default_class_labels_for_crop(crop_id)`; used in `apple_classifier.py` and `train_classifier.py`.
- Env: `CV_CLASS_LABELS_PATH` (Docker: `/config/cv_class_labels.json`).

For a new CV crop: add label array and dataset folders with the same names.

---

## `prompts.json`

Key — `crop_id`, fields:

| Field | Used in |
|-------|---------|
| `rag_system` | system message for text RAG LLM |
| `rag_task_intro` | `<system>` block in RAG user prompt |
| `photo_system` | system for photo advice |
| `photo_user_intro` | intro in photo user prompt |
| `photo_user_body` | prompt body: class, confidence, top-3 (`fmt.Sprintf` with `%s`, `%.2f%%`, `%v`) |

Load: `loadPromptCatalog()` on Go startup (`crops.go`), `promptsForCrop(cropID)` in `rag_chat.go` and `photo_recommendations.go`.

Env: `PROMPTS_CONFIG_PATH` (server: `/config/prompts.json`).

---

## `photo_templates.json`

Key — CV class label (`healthy_apple`, `apple_scab`, …) or **`default`** (placeholders `{{PREDICTION}}`, `{{CONFIDENCE}}`).

Load: `loadPhotoTemplates()` in `main.go` (`photo_templates.go`).

Env: `PHOTO_TEMPLATES_PATH` (default `config/photo_templates.json`, in Docker: `/config/photo_templates.json`).

In `docker-compose.yml` folder `./config` is mounted to `/config` (server and classifier). Go reload without restart: `docker compose kill -s HUP server` or `CONFIG_RELOAD_INTERVAL_SEC`.

Hard RAG rules (no invention, no article names) — in `rag_verify.go` / `rag_chat.go`, not this JSON.

---

## `branding.json`

Web App texts (domain pack): `app_title`, `header_emoji`, `header_subtitle`, `crop_label`, `onboarding_title`, `chat_divider`, `disclaimer`, **`photo_beta_notice`**.

- `photo_beta_notice` — CV beta warning; shown in UI when attaching photo and appended to recommendation in Go (`classify_flow.go`).

- Load: `loadBrandingConfig()` in `main.go` (`branding.go`).
- API: `GET /branding`, `GET /api/branding` (public).
- Web App: `loadBranding()` in `app.js` on startup.

Env: `BRANDING_CONFIG_PATH` (Docker: `/config/branding.json`).

When cloning the platform for another business change **only this file** (and `webapp/` if needed), not Go.

---

## `question_categories.json`

Keywords per category (`rootstock`, `disease`, `fertilizer`, …) for `classify_question()` in Python.

- File: `config/question_categories.json` (domain pack).
- Env: `QUESTION_CATEGORIES_CONFIG_PATH`.
- See [rag-retrieval.md](./rag-retrieval.md), `tests/test_question_categories.py`.

---

## `few_shot.json`

Structure: `crop_id` → category → example string.

Categories from `classify_question()` in [rag-retrieval.md](./rag-retrieval.md) via keywords in **`config/question_categories.json`**.

Example for apple (`disease`): typical answer tone with numbers from articles.

**Edit** when improving LLM answer quality without changing Python.

---

## `onboarding.json`

Array of question strings per crop.  
`GET /api/onboarding?crop_id=apple` → chips in chat before first message.

File search paths: `ONBOARDING_CONFIG_PATH` or `/config/onboarding.json` (in server image `COPY config /config`).

---

## `article_titles.json`

Map `filename.txt` → long title for index metadata and LLM context (“Text from article '…'”).

If file missing — filename is used as-is.  
**Not** shown to user in chat as “Source: …” (disclaimer policy).

---

## How to apply changes

| Service | Action after JSON edit |
|---------|------------------------|
| **crops / prompts / onboarding / photo_templates** | `docker compose restart server` (folder `/config` in image; hot-reload without rebuild needs volume — config is in image now) |
| **few_shot / article_titles** | `restart classifier` or reindex not needed for few_shot; for titles — **reindex** if only titles changed |
| **New articles in data/** | reindex — [data-pipeline.md](./data-pipeline.md) |

After changing `crops.json` in dev also reset Python cache: restart classifier service.

---

## Relation to `.env.example`

Configs do not duplicate secrets. In `.env` only paths if needed:

- `CROPS_CONFIG_PATH`, `PROMPTS_CONFIG_PATH`, `ONBOARDING_CONFIG_PATH`, `PHOTO_TEMPLATES_PATH`

---

## Brief summary

`config/` — **product behavior without Go/Python**: which crops are active, how the agronomist speaks, UI examples and few-shot. First place for content edits before heavy code changes.
