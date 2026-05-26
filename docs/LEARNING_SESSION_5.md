# Сессия 5 — UX, feedback и минимальная админка

## Что сделали

1. **Онбординг** — `config/onboarding.json`, `GET /api/onboarding?crop_id=`, чипы с примерами вопросов в Web App.
2. **Feedback 👍/👎** — миграция `003_feedback_analytics.sql`, `POST /api/feedback`, оценки в UI по `message.id`.
3. **Аналитика** — таблица `analytics_events`, события `message_sent`, `rag_answer`, `photo_classified`, `message_feedback`.
4. **Админка** — Basic auth (`ADMIN_USER` / `ADMIN_PASSWORD`):
   - `GET /api/admin/articles?crop_id=`
   - `POST /api/admin/upload` — `.txt` в `data/{crop_id}/`
   - `POST /api/admin/reindex` → Python `POST /admin/reindex` с `X-Admin-Secret`
5. **Web** — `webapp/admin.html` для загрузки статей и переиндексации.

## Зачем (обучение)

| Идея | Реализация |
|------|------------|
| **Product loop** | Онбординг снижает «пустой экран», feedback даёт сигнал качества RAG |
| **Event log** | JSONB в Postgres — проще, чем сразу ClickHouse; без PII в payload |
| **Admin без CMS** | Загрузка `.txt` + reindex — минимум для контент-менеджера |
| **Разделение секретов** | Basic auth для людей, `ADMIN_SECRET` для server→Python |
| **ID сообщений** | `AppendMessage` возвращает `id` — UI может ссылаться на конкретный ответ |

## Настройка `.env`

```env
ADMIN_USER=admin
ADMIN_PASSWORD=your-strong-password
ADMIN_SECRET=random-shared-secret
```

`ADMIN_SECRET` должен быть одинаковым у сервисов **server** и **classifier**.

## Проверка

1. Откройте чат → при пустой истории видны **примеры вопросов**; клик отправляет вопрос.
2. После ответа ассистента нажмите **👍** или **👎** — повторный клик заблокирован.
3. Откройте `http://localhost/admin.html` → логин → загрузите `.txt` в `apple` → **Переиндексировать RAG**.
4. Задайте вопрос по новой статье.

## SQL для аналитики (пример)

```sql
SELECT event_type, COUNT(*) FROM analytics_events
WHERE created_at > NOW() - INTERVAL '7 days'
GROUP BY event_type;

SELECT rating, COUNT(*) FROM message_feedback GROUP BY rating;
```

## Следующая сессия (6)

Тесты (Go/Python), smoke-скрипты, GitHub Actions CI.
