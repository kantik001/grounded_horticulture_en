# Дорожная карта doctor_gardens_ai

Актуальный план развития продукта. Отмечайте выполненное по мере работы.

## Фаза 0 — Подготовка
- [x] `.env.example`
- [x] README (актуальный flow)
- [x] `docs/ROADMAP.md`
- [x] `docs/ARCHITECTURE.md` — ядро платформы vs domain pack
- [x] `docs/DEPLOY.md` — развёртывание и клонирование

## Фаза 1 — Фундамент
### 1A Безопасность
- [x] Telegram `initData` validation (Go)
- [x] Web App передаёт `X-Telegram-Init-Data`
- [x] Защита API (кроме `/health`)
- [x] CORS whitelist
- [x] Rate limit (in-memory)
- [ ] Структурированные логи

### 1B Персистентность
- [x] PostgreSQL в docker-compose
- [x] Миграции `users`, `sessions`, `messages`
- [x] Store в Go, убрать in-memory сессии
- [x] Фото на volume, token в БД (не base64)

### 1C Прочее
- [x] Дисклеймер в UI
- [x] Не логировать тело LLM

## Фаза 2 — Мультикультура
- [x] `crop_id` в API и UI
- [x] `data/{crop}/`, RAG metadata
- [x] Модели CV по культуре (registry, apple)
- [x] Промпты/few-shot по культуре (config/)

## Фаза 3 — Качество RAG
- [x] 15 статей на яблоню (`data/apple/`, включая article4–15)
- [x] Скрипт переиндексации (`scripts/reindex_rag.py`, admin reindex)
- [x] Feedback 👍/👎
- [x] Sandbox-домен `demo_hr` (RAG без CV) — проверка универсальности
- [ ] Qdrant (при росте объёма)

### 3B — Eval
- [x] Набор **30 вопросов** по яблоне (`eval/rag_apple_baseline.jsonl`)
- [x] Mini-eval **demo_hr** (`eval/rag_demo_hr_baseline.jsonl`)
- [x] `scripts/run_rag_eval.py` + `eval/results/`, `make eval-retrieval`
- [ ] Прогон eval в CI (опционально, нужен classifier в job)
- [ ] Manual score 1–5 выборочно после пилота
- Документация: [`docs/knowledge-base/quality-eval-and-rag-logs.md`](knowledge-base/quality-eval-and-rag-logs.md), [`eval/README.md`](../eval/README.md)

### 3C — Логи RAG
- [x] Структурированный лог `[RAG]` в Go (`rag_log.go`): crop_id, session_id, fragments, verify
- [ ] Связка с **feedback** 👍/👎 в админ-отчёте
- [x] Без полного тела LLM в логах (политика 1C)

### Платформа (параллельно агро)
- [x] `config/branding.json` + `GET /branding` + загрузка в Web App
- [x] [`docs/ARCHITECTURE.md`](ARCHITECTURE.md), [`docs/DEPLOY.md`](DEPLOY.md)

## Фаза 4 — Vision
- [ ] Датасет и обучение `apple_classifier.pth`
- [ ] Метрики, порог confidence
- [ ] RAG + CV связка

## Фаза 5–10
См. обсуждение в чате: UX, админка, монетизация, тесты/CI, пилот, агрономы, IoT.

### Сессия 5 (UX + admin)
- [x] Онбординг (`config/onboarding.json`)
- [x] Feedback и analytics в Postgres
- [x] Admin upload + RAG reindex
- [x] `docs/LEARNING_SESSION_5.md`

### Сессия 6 (тесты + CI)
- [x] Go unit-тесты (RAG verify, crops, admin)
- [x] Python pytest (verifier, crops_config)
- [x] Smoke-скрипты
- [x] GitHub Actions CI
- [x] `docs/LEARNING_SESSION_6.md`
