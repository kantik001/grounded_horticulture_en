# ЧЕРНОВИК — серия Habr, часть 6/7

---

# Docker, auth, метрики и админка: что нужно, чтобы RAG показать работодателю за 15 минут

**Кратко:** четыре контейнера, два способа входа, Prometheus-метрики, админка с feedback — и две war story из реального демо.

*Серия grounded-horticulture. Финал практической части перед [частью 7](./07_platform_domain_pack.md).*

---

## Docker Compose: четыре сервиса

```yaml
# docker-compose.yml (упрощённо)
services:
  postgres:    # сессии, feedback, analytics
  classifier:  # Python :5000 — /rag/context, /classify
  server:      # Go :8080 — /message, auth, LLM
  webapp:      # nginx :80 — index.html, admin.html
```

Volumes:

- `chroma_data`, `bm25_data` — индексы RAG (переживают restart);
- `postgres_data`, `uploads_data` — диалоги и фото.

После смены статей:

```bash
make docker-reindex-apply
```

Reindex **внутри** classifier volume; на Windows локальный reindex без Docker часто ломается на Chroma — в README это явно сказано.

Healthchecks у postgres, classifier, server — `docker compose ps` должен показывать `healthy` перед демо.

---

## Auth: Telegram и браузер

Защищённые маршруты требуют **одно из**:

1. `X-Telegram-Init-Data` — подпись Web App (HMAC с токеном бота).
2. `X-API-Key` — ключ из `API_KEYS` в `.env`.

```go
func combinedAuthMiddleware(cfg *Config) gin.HandlerFunc {
    key := c.GetHeader("X-API-Key")
    if key != "" {
        rec, ok := lookupAPIKey(key)
        // actor_id стабилен для сессий браузера
        c.Next()
        return
    }
    telegramAuthMiddleware(cfg)(c)
}
```

Для локальной разработки: `TELEGRAM_AUTH_DISABLED=true` — **только dev**, в README и `.env.example` с предупреждением.

Rate limit: `RATE_LIMIT_REQUESTS_PER_MINUTE` (in-memory, с GC устаревших ключей).

---

## Миграции Postgres

SQL в `migrations/001_init.sql` … `003_feedback_analytics.sql`. При старте Go-сервера — `runAllMigrations()`:

- ищет `MIGRATIONS_DIR` (`/migrations` в Docker);
- применяет все `.sql` по имени;
- **нет** таблицы `schema_migrations` — для пилота достаточно; на проде лучше добавить учёт версий.

Документация: `docs/knowledge-base/migrations-overview.md`.

---

## Метрики и алерты

`GET /metrics` — Prometheus text format, **без auth** (на проде — firewall).

Примеры:

- `garden_http_requests_total`
- `garden_rag_verify_fail_total`
- `garden_rag_retrieval_ms_total` / `garden_rag_requests_total` → средняя latency retrieval

Документ с PromQL: `docs/knowledge-base/metrics-and-alerts.md`.

---

## Админка

`http://localhost/admin.html` — Basic Auth (`ADMIN_USER` / `ADMIN_PASSWORD`):

- список статей по `crop_id`;
- upload `.txt` + reindex;
- вкладка **обратная связь** — 👍/👎 с вопросом, ответом и rag metadata (`retrieval_ms`, `category`, `verify_pass`).

Полезно на собеседовании: показать 👎 и объяснить, как дебажить retrieval по логу.

---

## War story 1: gunicorn и PyTorch fork

**Симптом:** healthcheck OK, первый `POST /rag/context` висит минутами.

**Причина:** `preload_app=True`, прогрев моделей в master до fork worker'а — deadlock с sentence-transformers.

**Фикс** (`api/gunicorn.conf.py`):

```python
preload_app = False

def post_fork(server, worker):
    from rag.warmup import warmup_rag
    warmup_rag()
```

Прогрев загружает Chroma, e5, reranker и делает тестовый запрос (~30 с при старте).

---

## War story 2: few_shot.json в Docker

Config монтируется в `/config/`, код искал `/app/config/few_shot.json`. Warmup падал, RAG без few-shot.

Фикс — поиск пути как у `crops_config`:

```python
for candidate in (
    os.path.join(_PROJECT_ROOT, "config", "few_shot.json"),
    "/config/few_shot.json",
):
```

Мораль: в compose всегда проверять **mount paths** для JSON domain pack.

---

## Демо за 15 минут (чеклист)

1. Заранее `docker compose up` + дождаться `RAG warmup: готово`.
2. Браузер: API-ключ из `API_KEYS` (только ключ, без `:label`).
3. Вопросы: парша, подвой СК, марссониоз.
4. Админка: feedback, upload sample-статьи.
5. Опционально: `curl localhost:8080/metrics | head`.

Не показывать CV как основной сценарий — бета без весов.

---

## Что не делал (и почему)

- Kubernetes, MLflow, OpenTelemetry — P2–P3 бэклог.
- JSON structured logging — пока `log.Printf` и `[RAG]` строки достаточно для портфолио.
- golangci-lint в CI — есть тесты, линтер можно добавить позже.

---

## Дальше

[Часть 7](./07_platform_domain_pack.md) — платформа и `demo_hr`.  
[DEPLOY.md](https://github.com/kantik001/grounded-horticulture_ru/blob/main/docs/DEPLOY.md) в репозитории.

---

## Заметки автору

**Хабы:** DevOps, Go, Python  
**Картинки:** `docker compose ps`, скрин админки feedback  
**Объём:** ~8–10 тыс. знаков
