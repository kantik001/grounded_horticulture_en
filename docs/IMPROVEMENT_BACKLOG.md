# Бэклог улучшений doctor_gardens_ai

Создан по итогам аудита кода и документации.

**Принцип остановки:** пилот текстового RAG запускается после закрытия P0.
P1–P3 — пост-пилотный бэклог, не блокируют запуск. Признак перебора: задача не
влияет на решение «запускать пилот или нет» — значит, она за границей MVP.

Статусы: 🔲 не начато · 🔄 в работе · ✅ готово

См. также: [PILOT_READINESS_AUDIT.md](./PILOT_READINESS_AUDIT.md), [ROADMAP.md](./ROADMAP.md), [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## P0 — обязательно (дёшево, снижает риск / разблокирует пилот)

| # | Статус | Задача | Ветка | Файлы | Критерий готовности |
|---|--------|--------|-------|-------|---------------------|
| 1 | 🔄 | Timing-safe сравнение секретов | `feat/post-audit-p0` | `server/admin.go`, `api/app.py` | `subtle.ConstantTimeCompare`/`hmac.Equal` (Go), `hmac.compare_digest` (Py); тесты зелёные |
| 2 | 🔄 | Актуализировать аудит готовности | `feat/post-audit-p0` | `docs/PILOT_READINESS_AUDIT.md` | Метрики/статусы соответствуют текущему состоянию репозитория |
| 3 | ✅ | Решение по фото-ветке (CV): помечено «бета» | `feat/post-audit-p0` | `config/branding.json`, `server/branding.go`, `server/classify_flow.go`, `webapp/*` | Дисклеймер добавляется к каждой рекомендации по фото + виден в UI при прикреплении. Обучение модели — пост-пилотная задача (нет датасета) |

## P1 — качество и чистота платформы

| # | Статус | Задача | Ветка | Детали |
|---|--------|--------|-------|--------|
| 4 | 🔲 | Вынести доменные ключевые слова из ядра | `refactor/question-categories-to-config` | `classify_question` в `rag/retrieval.py` → категории/keywords в `config/` (domain pack) |
| 5 | 🔲 | Контрактный тест verify (Go ↔ Python) | `test/verify-contract` | Общий JSON-набор кейсов сверяет `rag_verify.go` и `rag/verifier.py` |
| 6 | 🔲 | Документировать ограничения верификации | `docs/verify-limits` | Явно: проверка только наличия числа во фрагментах, не привязки к контексту |
| 7 | 🔲 | OCR-починка PDF-текстов | `fix/ocr-corpus` | Поднять recall по «битым» словам (P1 из ROADMAP) |

## P2 — эксплуатация и наблюдаемость

| # | Статус | Задача | Ветка | Детали |
|---|--------|--------|-------|--------|
| 8 | 🔲 | GC устаревших ключей в rate-limiter | `fix/ratelimit-gc` | `server/ratelimit.go`: чистка stale counters (утечка памяти) |
| 9 | 🔲 | Вернуть rag-eval в CI | `feat/ci-rag-eval` | Объединить build+eval в один job (`.github/workflows/ci.yml`) |
| 10 | 🔲 | `/metrics` + базовые алерты | `feat/metrics` | Ошибки LLM, латентность retrieval/LLM |
| 11 | 🔲 | Связка feedback ↔ RAG-логи в админ-отчёте | `feat/feedback-rag-report` | ROADMAP 3C |
| 12 | 🔲 | Документировать бэкапы volumes | `docs/backups` | `chroma_data`, `bm25_data`, `postgres_data` + расписание |

## P3 — тесты и устойчивость

| # | Статус | Задача | Ветка | Детали |
|---|--------|--------|-------|--------|
| 13 | 🔲 | HTTP-тесты Go-хендлеров | `test/http-handlers` | `httptest` для `/message`, auth, rate limit |
| 14 | 🔲 | Интеграционные тесты Postgres | `test/pg-integration` | testcontainers для `postgres_store.go` |
| 15 | 🔲 | Глобальный лимит размера JSON-тела | `fix/body-size-limit` | `MaxBytesReader` на не-multipart запросы |

---

## Определение «достаточно» (Definition of Done для пилота)

Останавливаемся, когда закрыт P0-чеклист из [PILOT_READINESS_AUDIT.md](./PILOT_READINESS_AUDIT.md)
плюс пункты 1–3 выше.
