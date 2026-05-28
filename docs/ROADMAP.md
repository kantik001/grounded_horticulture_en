# Дорожная карта doctor_gardens_ai

Актуальный план развития продукта. Отмечайте выполненное по мере работы.

## Фаза 0 — Подготовка
- [x] `.env.example`
- [x] README (актуальный flow)
- [x] `docs/ROADMAP.md`
- [ ] `docs/ARCHITECTURE.md`

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
- [ ] 15–25 статей на яблоню
- [x] Скрипт переиндексации (`scripts/reindex_rag.py`, admin reindex)
- [x] Feedback 👍/👎
- [ ] Qdrant (при росте объёма)

### 3B — Eval (план)
- [ ] Набор **30–50 вопросов** с эталонами / критериями по яблоне
- [ ] Прогон eval после **reindex** и при смене модели/промпта
- [ ] Метрики: verify pass rate, «нет в материалах», выборочный manual score
- [ ] (Опционально) скрипт `scripts/run_rag_eval.py` + `eval/results/`
- Документация: [`docs/knowledge-base/quality-eval-and-rag-logs.md`](knowledge-base/quality-eval-and-rag-logs.md)

### 3C — Логи RAG (план)
- [ ] Логировать: вопрос → `crop_id` → top-k фрагменты → verify pass/fail → `message_id`
- [ ] Связка с **feedback** 👍/👎 для разбора плохих ответов
- [ ] Без полного тела LLM в логах (политика 1C)
- Документация: [`docs/knowledge-base/quality-eval-and-rag-logs.md`](knowledge-base/quality-eval-and-rag-logs.md)

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
