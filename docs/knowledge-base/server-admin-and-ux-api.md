# Walkthrough: admin and UX API (`server/`)

**Files:** `admin.go`, `onboarding.go`, `feedback.go`, `analytics_store.go`, `crops.go`  
**Client:** [webapp-overview.md](./webapp-overview.md) (`admin.html`, `index.html`)

---

## Overview

Small handlers that do not run ML themselves:

- **admin** — articles on disk + reindex;
- **crops / onboarding** — config for UI;
- **feedback + analytics** — product metrics.

---

## `admin.go` — RAG article management

### Auth `adminBasicAuth`

- HTTP Basic: `ADMIN_USER` / `ADMIN_PASSWORD`.
- If `ADMIN_PASSWORD` empty → **503** “admin disabled”.

**Not** Telegram initData.

### Routes (duplicate `/admin` and `/api/admin`)

| Method | Handler | Action |
|--------|---------|--------|
| GET | `handleAdminStatus` | `{ data_dir, ok }` |
| GET | `handleAdminListArticles` | list `.txt` in `data/{crop_id}/` |
| POST | `handleAdminUpload` | save file |
| POST | `handleAdminReindex` | `triggerRAGReindex` → Python |
| GET | `handleAdminFeedback` | 👍/👎 ratings with question, answer, and **`rag`** field |

### Upload

- `crop_id` from form.
- File: regex `^[a-zA-Z0-9._-]+\.txt$`, max **2 MB**.
- Path: `{DATA_DIR}/{crop_id}/{filename}`.

Test: `admin_test.go` — `TestSafeFilename`.

### Reindex

HTTP POST to `{PYTHON_BASE_URL}/admin/reindex` with header **`X-Admin-Secret`** = `ADMIN_SECRET`.

Resets Chroma + BM25 in Python — see [rag-vector_store.md](./rag-vector_store.md).

---

## `crops.go` — crop catalog

### Load on startup

`loadCropCatalog()` reads `CROPS_CONFIG_PATH` or `config/crops.json` (same meaning as Python `crops_config`).

### `GET /crops`, `/api/crops` — public

No Telegram auth. Response:

```json
{
  "success": true,
  "default_crop": "apple",
  "crops": [
    { "id": "apple", "name_ru": "Apple", "emoji": "🍎", "cv_enabled": true, "rag_enabled": true }
  ]
}
```

`normalizeCropID` / `getCropMeta` — used in chat and RAG handlers.

---

## `onboarding.go` — sample questions

### Config

`config/onboarding.json` — map `crop_id` → array of question strings.

`ONBOARDING_CONFIG_PATH` in Docker: `/config/onboarding.json`.

### `GET /onboarding?crop_id=apple` — public

```json
{ "success": true, "crop_id": "apple", "questions": ["What are scab symptoms?", ...] }
```

Web App renders chips; click → `sendMessage()`.

---

## `feedback.go` — answer ratings

### `POST /feedback` (protected)

JSON:

```json
{ "session_id": "...", "message_id": 123, "rating": 1 }
```

`rating`: **1** (👍) or **-1** (👎).

- Check: message exists and belongs to user.
- `UNIQUE (message_id, user_id)` in DB — one vote per message.
- `LogEvent("message_feedback", ...)`.

Table: `message_feedback` — [migrations-overview.md](./migrations-overview.md).

### `GET /admin/feedback` (Basic auth)

Query: `rating` (1 or -1), `limit` (default 50).

Response: list of ratings with `question`, `answer`, `rating`, and optional **`rag`** — metadata from `analytics_events` (`rag_answer`): category, fragments, verify_pass, latency.  
See [metrics-and-alerts.md](./metrics-and-alerts.md), `server/feedback_report.go`.

---

## `analytics_store.go` — events

### `LogEvent(userTelegramID, eventType, payload)`

INSERT into `analytics_events` (`event_type`, `payload` JSONB).

Called from:

- `feedback.go`
- `logAnalytics` in `message_handlers.go` (`rag_answer`, `photo_classified`)

Example SQL analytics — tables `analytics_events`, `message_feedback` (see [migrations-overview.md](./migrations-overview.md)).

---

## Component relationships

```mermaid
flowchart LR
    UI[index.html / admin.html]
    Go[server handlers]
    Disk[data/]
    Py[classifier reindex]
    PG[(Postgres)]

    UI -->|/api/crops onboarding| Go
    UI -->|/api/message feedback| Go
    UI -->|/api/admin/*| Go
    Go --> Disk
    Go --> Py
    Go --> PG
```

---

## Env for this group

| Variable | File |
|----------|------|
| `ADMIN_USER`, `ADMIN_PASSWORD`, `ADMIN_SECRET` | admin |
| `DATA_DIR` | admin upload |
| `CROPS_CONFIG_PATH` | crops |
| `ONBOARDING_CONFIG_PATH` | onboarding |

---

## Brief summary

**admin.go** — RAG content on disk. **crops/onboarding** — public UX config. **feedback/analytics** — answer quality and telemetry. All around main chat from [server-chat-and-db.md](./server-chat-and-db.md), without duplicating ML logic.
