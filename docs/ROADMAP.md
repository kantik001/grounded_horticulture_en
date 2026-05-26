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
- [ ] `crop_id` в API и UI
- [ ] `data/{crop}/`, RAG metadata
- [ ] Модели CV по культуре
- [ ] Промпты/few-shot по культуре

## Фаза 3 — Качество RAG
- [ ] 15–25 статей на яблоню
- [ ] Скрипт переиндексации
- [ ] Feedback 👍/👎
- [ ] Qdrant (при росте объёма)

## Фаза 4 — Vision
- [ ] Датасет и обучение `apple_classifier.pth`
- [ ] Метрики, порог confidence
- [ ] RAG + CV связка

## Фаза 5–10
См. обсуждение в чате: UX, админка, монетизация, тесты/CI, пилот, агрономы, IoT.
