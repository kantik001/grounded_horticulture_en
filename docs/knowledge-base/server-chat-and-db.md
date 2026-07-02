# Walkthrough: chat and database (`server/`)

**Files:** `message_handlers.go`, `session_handlers.go`, `chat_session.go`, `postgres_store.go`, `classify_flow.go` (photos)  
**DB:** schema in [migrations-overview.md](./migrations-overview.md)  
**Client:** [webapp-overview.md](./webapp-overview.md) → `POST /message`

---

## Main user flow

**`POST /message`** (auth + rate limit) — everything the Telegram chat does.

Two body formats:

| Content-Type | Fields | Branch |
|--------------|--------|--------|
| `application/json` | `session_id`, `crop_id`, `text` | text → RAG |
| `multipart/form-data` | `session_id`, `crop_id`, `text`, `image` | photo → CV + LLM |

Photo limit: **10 MB** (`maxUploadImageBytes` in `classify_flow.go`).

---

## `handleMessage` → branching

```mermaid
flowchart TD
    M[POST /message] --> P{multipart?}
    P -->|yes| I[handleImageMessage]
    P -->|no| T[handleTextMessage]
    T --> RAG[answerWithRAG + history]
    I --> SAVE[SaveImage token]
    I --> CAR[classifyAndRecommend]
    CAR --> CV[sendToClassifier]
    CAR --> LLM[generatePhotoRecommendation]
```

Response always: JSON `{ success, session_id, crop_id, messages: [...] }` — full history for UI.

---

## Text: `handleTextMessage`

1. **`HistoryForLLM`** — latest user/assistant turns for LLM context (up to 80 messages in store).
2. **`answerWithRAG(text, cropID, prior)`** — see [server-rag_chat.md](./server-rag_chat.md).
3. Save **user** message in DB.
4. On RAG error (soft) — assistant with error text, `logAnalytics("rag_answer", soft_fail)`.
5. On success — assistant with answer, analytics `soft_fail: false`.
6. **`respondWithMessages`** — return updated message list.

---

## Photo: `handleImageMessage`

1. History for LLM (same as text).
2. **`SaveImage`** — file in `UPLOAD_DIR`, DB stores only **token** (not base64).
3. **`classifyAndRecommend(image, cropID, caption, prior)`** (`classify_flow.go`):
   - Python CV → prediction + confidence;
   - **`generatePhotoRecommendation`** — LLM with history or template from `photo_templates.json`.
4. User message: caption, `kind=image`, token, class_prediction, class_confidence.
5. Assistant message with recommendation.
6. Analytics `photo_classified` with prediction/confidence.

On CV failure — assistant with error, no LLM.

Separate **`POST /classify`** (no session) uses same `classifyAndRecommend` but without DB save — see [server-overview.md](./server-overview.md).

---

## Sessions: `session_handlers.go` + `chat_session.go`

### `POST /session`

JSON `{ "crop_id": "apple" }` → new `chat_sessions` + `session_id` (random hex).

### `GET /history?session_id=`

Owner check (telegram_id) → message list for UI.

### `GET /media/:token`

Serve photo file from disk; only if token belongs to user session.

### `ctxTelegramUser(c)`

Gets `TelegramUser` from Gin context after middleware.

---

## `postgres_store.go` — `ChatStore`

### Connection

- `pgxpool` to `DATABASE_URL`.
- On startup: migrations + long-lived pool.

### Key methods

| Method | Purpose |
|--------|---------|
| `UpsertUser` | user by `telegram_id` |
| `CreateSession` / `GetOrCreateSession` | session + crop_id |
| `sessionOwned` | foreign session_id → 404 |
| `AppendMessage` | INSERT into `messages` |
| `ListMessages` | history + LEFT JOIN `message_feedback.rating` |
| `HistoryForLLM` | role/content for LLM |
| `SaveImage` | file + token |
| `OpenImageForUser` | safe media serve |
| `SaveMessageFeedback` | 👍/👎, UNIQUE (message_id, user_id) |
| `LogEvent` | INSERT `analytics_events` JSONB |

### Session security

Any request with `session_id` checks: session belongs to **this** `telegram_id`. Cannot read another user’s chat by guessing id.

### History limit

`maxSessionMessages = 80` — trim on fetch for LLM (not deleted from DB).

---

## `ChatMessage` structure (for API)

JSON fields for webapp:

- `id`, `role`, `content`, `kind`
- `image_url` or path via `/media/:token`
- `class_prediction`, `class_confidence` (for photos)
- `feedback_rating` (-1, 1 or null)

---

## `POST /chat` vs `POST /message`

| | `/chat` | `/message` |
|--|---------|------------|
| Status | **deprecated** (`Deprecation: true`) | main API |
| DB history | no | yes |
| Session | not required | required |
| Use | legacy integrations | Telegram Web App |

Web App sends only **`POST /api/message`**. Both text paths call `answerWithRAG` (with `rag_enabled` check); `/message` saves dialog in Postgres.

Photo in chat: multipart `image` + `readImageFromFormFile` → `classifyAndRecommend` (`cv_enabled` check).

---

## Analytics from message handlers

`logAnalytics` in `message_handlers.go` → `analytics_events` (see [server-admin-and-ux-api.md](./server-admin-and-ux-api.md)):

- `rag_answer` — RAG success/soft_fail
- `photo_classified` — CV result

---

## Common errors

| Symptom | Where to look |
|---------|---------------|
| “Session not found” | wrong session_id or user change |
| Photo missing in chat | `loadAuthedImage` + token + auth |
| Empty history after refresh | `GET /history`, sessionStorage in UI |
| DB error on startup | postgres not ready, migrations |

---

## Brief summary

**message_handlers** / **session_handlers** — chat business logic (text/photo). **postgres_store** — persistence and user isolation. **chat_session** — Telegram user helpers from context. Center of the product for the gardener in Telegram.
