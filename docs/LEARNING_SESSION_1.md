# Сессия 1 — что мы сделали и зачем (обучение)

## Шаг 0.1 — `.env.example`

**Зачем:** секреты не хранят в git. В репозитории — шаблон переменных, у каждого разработчика свой `.env`.

**Навык для работы:** 12-factor app, configuration via environment.

## Шаг 0.2 — README

**Зачем:** документация совпадает с кодом (Python = retrieval, Go = LLM + auth).

## Шаги 1.1–1.3 — Telegram `initData`

**Как работает Mini App:**

1. Telegram открывает ваш `index.html` внутри клиента.
2. В JS доступен `Telegram.WebApp.initData` — строка вида `query_id=...&user=...&auth_date=...&hash=...`.
3. `hash` — HMAC подпись от Telegram. Подделать без токена бота нельзя.
4. Браузер отправляет эту строку на ваш Go API в заголовке `X-Telegram-Init-Data`.
5. Go пересчитывает HMAC с `TELEGRAM_BOT_TOKEN` и сравнивает с `hash`.

**Файлы:** `server/auth_telegram.go`, `server/middleware.go`, `webapp/index.html` (`withAuthHeaders`).

**Навык:** authentication vs authorization; не доверять `user_id` с клиента без проверки подписи.

**Dev-режим:** `TELEGRAM_AUTH_DISABLED=true` — только локально.

## Шаг 1.4 — CORS

**Зачем:** браузер блокирует запросы с чужого домена. Раньше было `*` — любой сайт мог дергать API из браузера жертвы.

**Сейчас:** список `CORS_ALLOWED_ORIGINS`; отражаем только разрешённый `Origin`.

## Шаг 1.5 — Rate limit

**Зачем:** один пользователь не может исчерпать бюджет LLM тысячами запросов.

**Сейчас:** in-memory счётчик на `telegram_user_id` (30/мин по умолчанию). На нескольких серверах позже — Redis.

## Дисклеймер в UI

Юридический минимум для советов по болезням и препаратам.

## Проверка

```bash
cp .env.example .env
# для локали без Telegram:
# TELEGRAM_AUTH_DISABLED=true

docker compose up --build
```

Тесты Go (если установлен Go):

```bash
cd server && go test ./...
```

## Следующая сессия (2)

PostgreSQL, таблицы users/sessions/messages, убрать хранение истории в RAM.
