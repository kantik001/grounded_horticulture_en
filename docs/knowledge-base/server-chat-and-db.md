# Разбор: чат и база данных (`server/`)

**Файлы:** `messenger.go`, `chat_session.go`, `postgres_store.go`  
**БД:** схема в [migrations-overview.md](./migrations-overview.md)  
**Клиент:** [webapp-overview.md](./webapp-overview.md) → `POST /message`

---

## Главный пользовательский поток

**`POST /message`** (auth + rate limit) — всё, что делает чат в Telegram.

Два формата тела:

| Content-Type | Поля | Ветка |
|--------------|------|--------|
| `application/json` | `session_id`, `crop_id`, `text` | текст → RAG |
| `multipart/form-data` | `session_id`, `crop_id`, `text`, `image` | фото → CV + LLM |

Лимит фото: **10 МБ**.

---

## `handleMessage` → разветвление

```mermaid
flowchart TD
    M[POST /message] --> P{multipart?}
    P -->|да| I[handleImageMessage]
    P -->|нет| T[handleTextMessage]
    T --> RAG[answerWithRAG + history]
    I --> SAVE[SaveImage token]
    I --> CV[sendToClassifier]
    I --> LLM[generateRecommendationWithHistory]
```

Ответ всегда: JSON `{ success, session_id, crop_id, messages: [...] }` — полная история для UI.

---

## Текст: `handleTextMessage`

1. **`HistoryForLLM`** — последние реплики user/assistant для контекста LLM (до 80 сообщений в store).
2. **`answerWithRAG(text, cropID, prior)`** — см. [server-rag_chat.md](./server-rag_chat.md).
3. Сохранить сообщение **user** в БД.
4. При ошибке RAG (мягкой) — assistant с текстом ошибки, `logAnalytics("rag_answer", soft_fail)`.
5. При успехе — assistant с ответом, analytics `soft_fail: false`.
6. **`respondWithMessages`** — отдать обновлённый список сообщений.

---

## Фото: `handleImageMessage`

1. История для LLM (как у текста).
2. **`SaveImage`** — файл в `UPLOAD_DIR`, в БД только **token** (не base64).
3. **`sendToClassifier`** → Python — prediction + confidence.
4. Сообщение user: caption, `kind=image`, token, class_prediction, class_confidence.
5. **`generateRecommendationWithHistory`** — LLM или шаблон (`server/photo_recommendations.go`, вызов `llm.go`).
6. Сообщение assistant с рекомендацией.
7. Analytics `photo_classified` с prediction/confidence.

При сбое CV — assistant с ошибкой, без LLM.

---

## Сессии: `messenger.go` + `chat_session.go`

### `POST /session`

JSON `{ "crop_id": "apple" }` → новая `chat_sessions` + `session_id` (random hex).

### `GET /history?session_id=`

Проверка владельца (telegram_id) → список сообщений для UI.

### `GET /media/:token`

Отдача файла фото с диска; только если token принадлежит сессии пользователя.

### `ctxTelegramUser(c)`

Достаёт `TelegramUser` из Gin context после middleware.

---

## `postgres_store.go` — `ChatStore`

### Подключение

- `pgxpool` к `DATABASE_URL`.
- При старте: миграции + долгоживущий пул.

### Ключевые методы

| Метод | Назначение |
|-------|------------|
| `UpsertUser` | user по `telegram_id` |
| `CreateSession` / `GetOrCreateSession` | сессия + crop_id |
| `sessionOwned` | чужой session_id → 404 |
| `AppendMessage` | INSERT в `messages` |
| `ListMessages` | история + LEFT JOIN `message_feedback.rating` |
| `HistoryForLLM` | role/content для LLM |
| `SaveImage` | файл + token |
| `OpenImageForUser` | безопасная отдача media |
| `SaveMessageFeedback` | 👍/👎, UNIQUE (message_id, user_id) |
| `LogEvent` | INSERT `analytics_events` JSONB |

### Безопасность сессий

Любой запрос с `session_id` проверяет: сессия принадлежит **этому** `telegram_id`. Нельзя читать чужой чат по угаданному id.

### Лимит истории

`maxSessionMessages = 80` — обрезка при выборке для LLM (не удаление из БД).

---

## Структура `ChatMessage` (для API)

Поля в JSON для webapp:

- `id`, `role`, `content`, `kind`
- `image_url` или путь через `/media/:token`
- `class_prediction`, `class_confidence` (для фото)
- `feedback_rating` (-1, 1 или null)

---

## `POST /chat` vs `POST /message`

| | `/chat` | `/message` |
|--|---------|------------|
| История в БД | нет | да |
| Сессия | не нужна | обязательна |
| Использование | простой API | Web App |

Оба используют `answerWithRAG`, но messenger сохраняет диалог.

---

## Аналитика из messenger

`logAnalytics` → `analytics_events` (см. [server-admin-and-ux-api.md](./server-admin-and-ux-api.md)):

- `rag_answer` — успех/soft_fail RAG
- `photo_classified` — результат CV

---

## Типичные ошибки

| Симптом | Где смотреть |
|---------|----------------|
| «Сессия не найдена» | чужой session_id или смена user |
| Фото не в чате | `loadAuthedImage` + token + auth |
| Пустая история после refresh | `GET /history`, sessionStorage в UI |
| Ошибка БД при старте | postgres не ready, миграции |

---

## Краткий итог

**messenger** — бизнес-логика чата (текст/фото). **postgres_store** — персистентность и изоляция пользователей. **chat_session** — хелперы Telegram user из context. Это центр продукта для садовода в Telegram.
