# Сессия 2 — PostgreSQL и персистентность чата

## Что сделали

1. **PostgreSQL** в `docker-compose.yml` (сервис `postgres`, volume `postgres_data`).
2. **Миграция** `migrations/001_init.sql`: таблицы `users`, `chat_sessions`, `messages`.
3. **Go store** (`server/postgres_store.go`): пользователи, сессии, сообщения, обрезка истории до 80.
4. **Фото на диске** (`UPLOAD_DIR`), в БД только `image_token`; отдача через `GET /api/media/:token` с auth.
5. **Сессия привязана к Telegram user** — чужой `session_id` не прочитать.

## Зачем (обучение)

| Концепция | Где в проекте |
|-----------|----------------|
| **Реляционная БД** | Postgres вместо `map` в RAM |
| **Миграции** | SQL-файл применяется при старте сервера |
| **Foreign keys** | `messages.session_id → chat_sessions → users` |
| **Ownership** | JOIN с `telegram_id` перед выдачей истории/медиа |
| **Blob storage** | Большие файлы не в JSON/БД, а на volume |

## Схема данных

```
users (telegram_id UNIQUE)
  └── chat_sessions (id TEXT)
        └── messages (role, content, image_token, ...)
```

## Переменные окружения

- `DATABASE_URL` — строка подключения Postgres
- `UPLOAD_DIR` — каталог фото (в Docker: `/data/uploads`)

## Проверка

```bash
cp .env.example .env
# TELEGRAM_AUTH_DISABLED=true для локали

docker compose up --build
```

1. Отправьте сообщение → перезапустите `server` → история на месте.
2. `GET http://localhost:8080/health` → `"database": "ok"`.

## Следующая сессия (3)

`crop_id`, выбор культуры в UI, RAG по папкам `data/apple/`.
