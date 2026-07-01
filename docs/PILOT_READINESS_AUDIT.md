# Аудит готовности к пилоту

**Дата создания:** 2026-06-05 · **Обновлён:** 2026-07-01  
**Продукт:** doctor_gardens_ai (Telegram Web App, агробот)  
**Цель пилота:** текстовый RAG по статьям ПВЮР + опционально фото (яблоня)

Статусы: ✅ готово · ⚠️ частично / риск · ❌ блокер · 🔲 не начато

> **Обновление 2026-07-01.** По итогам код-аудита заведён
> [IMPROVEMENT_BACKLOG.md](./IMPROVEMENT_BACKLOG.md). Выполнено: timing-safe сравнение
> секретов (`admin.go`, `api/app.py`). Юнит-тесты: Go `ok`, Python **33 passed**.
> Eval retrieval доведён до 100% (коммит «Reach 100% RAG eval») — **переподтвердить
> прогоном на живом индексе перед пилотом** (см. команды P0 ниже).

---

## Резюме

| Направление | Готовность | Комментарий |
|-------------|------------|-------------|
| **Текст RAG (яблоня)** | **~85%** | eval 100% (см. обновление), hybrid+reranker, ~344 статьи |
| **Текст RAG (груша/слива)** | **~80%** | pear 8/8, plum 10/10; слива очищена от misc |
| **Фото CV** | **~25%** | Нет обученных `.pth` в репо — ImageNet fallback |
| **Инфра / Docker** | **~90%** | Compose, volumes Chroma+BM25, CI |
| **Безопасность прод** | **~60%** | Нужны prod-секреты, `TELEGRAM_AUTH_DISABLED=false` |
| **Монетизация / аналитика** | **~40%** | Feedback есть, отчёты по RAG-логам — в плане |

**Вердикт:** пилот **текстового чата по яблоне (и груше/сливе)** можно запускать после чеклиста P0 ниже. Пилот **«фото → болезнь»** без обучения CV — только с явным дисклеймером о низкой точности.

---

## P0 — обязательно перед пилотом

| # | Пункт | Статус | Действие |
|---|--------|--------|----------|
| 1 | `LLM_API_KEY` задан и стабильная модель | ⚠️ | В `.env` free-модель OpenRouter; для пилота лучше платная/стабильная RU-модель |
| 2 | `TELEGRAM_BOT_TOKEN` + `TELEGRAM_AUTH_DISABLED=false` | ❌ | В dev сейчас `true`; на проде обязательно выключить |
| 3 | `ADMIN_PASSWORD`, `ADMIN_SECRET`, `POSTGRES_PASSWORD` — сильные | ⚠️ | Заменить dev-значения перед публичным доступом |
| 4 | RAG переиндексирован (Chroma + BM25) в Docker volumes | ✅ | 14 554 фрагментов; reindex при **остановленном** classifier |
| 5 | `HF_TOKEN` в `.env` для classifier | ✅ | Ускоряет загрузку e5 + `BAAI/bge-reranker-base` |
| 6 | Eval retrieval ≥ целевого порога | ✅ | **100%** (коммит «Reach 100% RAG eval»); переподтвердить прогоном перед пилотом |
| 7 | `docker compose up` — все 4 сервиса healthy | ✅ | postgres, classifier, server, webapp |
| 8 | Smoke: `make smoke` или `scripts/smoke.ps1` | 🔲 | Прогнать после деплоя |
| 9 | Дисклеймер в UI и в ответах RAG | ✅ | `branding.json`, verifier, Go disclaimer |
| 10 | CORS / домен Telegram Web App | ⚠️ | Добавить prod URL в `CORS_ALLOWED_ORIGINS` |

### Команды P0 (после смены `data/` или RAG-кода)

```powershell
docker compose -p union_ai_apple stop classifier
docker compose -p union_ai_apple run --rm -e FORCE_RAG_REINDEX=true classifier python scripts/reindex_rag.py
docker compose -p union_ai_apple start classifier
python scripts/run_rag_eval.py --suite all --timeout 300
```

---

## P1 — качество RAG (желательно до/в первую неделю пилота)

| # | Пункт | Статус | Детали |
|---|--------|--------|--------|
| 11 | Корпус apple ~344 статьи | ✅ | Журнал ПВЮР |
| 12 | Корпус plum очищен (misc удалены) | ✅ | 75 misc удалено → ~108 статей; 10 mixed остались |
| 13 | E5 префиксы `query:`/`passage:` | ✅ | `rag/embeddings.py` |
| 14 | Chunking 650/80 + секции | ✅ | `rag/chunking.py` |
| 15 | BM25 hybrid + RRF | ✅ | `rag/bm25_store.py`, `rag/hybrid.py` |
| 16 | Cross-encoder reranker | ✅ | `BAAI/bge-reranker-base`; первый запрос ~1–3 мин с HF_TOKEN |
| 17 | Diversity top-k (max 2/статья, k=8) | ✅ | `vector_store.diversify_fragments` |
| 18 | Few-shot по категориям | ✅ | rootstock, disease, relief, … |
| 19 | Исправить eval fail «марссониоз» | ✅ | Устранён в рамках доведения eval до 100% |
| 20 | OCR-починка PDF-текстов | 🔲 | Отложено; влияет на recall по «битым» словам |
| 21 | Ручная выборочная оценка 10–20 ответов LLM | 🔲 | `--full` eval или чат с реальными садоводами |

---

## P2 — фото / CV (не блокер текстового пилота)

| # | Пункт | Статус | Детали |
|---|--------|--------|--------|
| 22 | `apple_classifier.pth` обучен на болезнях | ❌ | В репо нет `.pth`; CV = ImageNet backbone |
| 23 | Датасет фото болезней яблони | 🔲 | `cv/train_classifier.py`, структура `dataset/train/` |
| 24 | Порог confidence + шаблоны без LLM | ⚠️ | `photo_templates.json` есть; порог в roadmap |
| 25 | `cv_enabled: false` для pear/plum | ✅ | Только apple в UI для фото |
| 26a | Пометка фото «бета» + дисклеймер | ✅ | 2026-07-01: `photo_beta_notice` в `branding.json`, добавляется к каждой рекомендации (`classify_flow.go`) и виден в UI при прикреплении фото |

**Решение (2026-07-01):** фото-ветка оставлена включённой, но помечена «бета» с явным дисклеймером о необученной модели. Обучение `apple_classifier.pth` — пост-пилотная задача (нужен датасет; PlantVillage покрывает лишь 4 из 10 классов).

---

## P3 — платформа и эксплуатация

| # | Пункт | Статус | Детали |
|---|--------|--------|--------|
| 26 | PostgreSQL сессии и история | ✅ | migrations 001–003 |
| 27 | Rate limit | ✅ | in-memory, 30 req/min |
| 28 | Admin: upload `.txt` + reindex | ✅ | `admin.html`, `/admin/reindex` |
| 29 | Feedback 👍/👎 в БД | ✅ | analytics в Postgres |
| 30 | RAG structured logs `[RAG]` | ✅ | `server/rag_log.go` |
| 31 | Связка feedback ↔ RAG logs в отчёте | 🔲 | ROADMAP 3C |
| 32 | CI: pytest + go test + rag-eval | ✅ | `.github/workflows/ci.yml` |
| 33 | Backup volumes (`chroma_data`, `bm25_data`, `postgres_data`) | 🔲 | Документировать расписание |
| 34 | Мониторинг / алерты | 🔲 | Только healthchecks в compose |

---

## P4 — продукт и UX

| # | Пункт | Статус | Детали |
|---|--------|--------|--------|
| 35 | Онбординг-чипы вопросов | ✅ | `config/onboarding.json` |
| 36 | Выбор культуры в UI | ✅ | apple / pear / plum |
| 37 | `demo_hr` скрыт (`ui_hidden`) | ✅ | Sandbox платформы |
| 38 | README / DEPLOY / knowledge-base актуальны | ✅ | Обновлено 2026-06-05 |
| 39 | Пилотная группа 5–10 пользователей | 🔲 | Организационно |
| 40 | Сбор обратной связи раз в неделю | 🔲 | Процесс |

---

## Метрики на момент аудита

| Метрика | Значение |
|---------|----------|
| Статьи RAG | ~344 apple, ~42 pear, ~108 plum |
| Фрагментов в индексе | 14 554 |
| Eval retrieval (all) | **100%** (переподтвердить перед пилотом) |
| apple / pear / plum / demo_hr | 30/30 · 8/8 · 10/10 · 5/5 |
| Unit-тесты Python | 33 passed (2026-07-01) |
| Unit-тесты Go | `go test ./...` — ok (2026-07-01) |
| Misc plum (аудит) | 0 misc, 10 mixed |

---

## Минимальный чеклист «старт пилота завтра»

- [ ] `.env` prod: `TELEGRAM_AUTH_DISABLED=false`, сильные пароли
- [ ] `CORS_ALLOWED_ORIGINS` с URL Web App
- [ ] `HF_TOKEN` в `.env` (не в git)
- [ ] Reindex + restart classifier (volumes не пустые)
- [ ] `python scripts/run_rag_eval.py --suite all`
- [ ] `make smoke`
- [ ] Решение по фото: отключить CV в UI или дисклеймер «бета»
- [ ] 5 тестовых вопросов вручную в Telegram

---

## Связанные документы

- [IMPROVEMENT_BACKLOG.md](./IMPROVEMENT_BACKLOG.md)
- [ROADMAP.md](./ROADMAP.md)
- [DEPLOY.md](./DEPLOY.md)
- [eval/README.md](../eval/README.md)
- [knowledge-base/rag-hybrid-search.md](./knowledge-base/rag-hybrid-search.md)
- [knowledge-base/plum_miscategorized_audit.md](./knowledge-base/plum_miscategorized_audit.md)

---

*Файл обновлять после каждого крупного изменения KB, RAG-пайплайна или перед релизом пилота.*
