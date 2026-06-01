# Сессия 3 — мультикультура (`crop_id`)

## Что сделали

1. **`config/crops.json`** — список культур (apple активна, pear/plum «скоро»).
2. **`config/prompts.json`**, **`config/few_shot.json`**, **`config/article_titles.json`** — конфиг вместо хардкода.
3. **`data/apple/`** — статьи перенесены из `data/*.txt`.
4. **RAG** — поиск с фильтром `crop_id` в Chroma (`rag/vector_store.py`).
5. **CV** — `cv/registry.py`, поле `crop_id` в `/classify`.
6. **Go** — `server/crops.go`, `crop_id` в сессии (миграция `002_crop_id.sql`), RAG и classify.
7. **UI** — выпадающий список культуры, новая сессия при смене.

## Зачем (обучение)

| Идея | Реализация |
|------|------------|
| **Конфиг vs код** | Новая культура = JSON + папка `data/` + флаг enabled |
| **Metadata в векторной БД** | `crop_id` в каждом чанке → фильтр при search |
| **Multi-tenant light** | Сессия хранит `crop_id`, промпты разные |
| **Feature flags** | `cv_enabled` / `rag_enabled` без деплоя кода |

## После обновления с сессии 2

Старая `chroma_db` без `crop_id` не подходит для фильтра. **Один раз:**

```bash
# в .env
FORCE_RAG_REINDEX=true
docker compose up --build
```

Потом верните `FORCE_RAG_REINDEX=false`.

## Проверка

1. Выберите **Яблоня** → задайте вопрос по статьям → ответ со источником.
2. Выберите **Груша** → текст: сообщение «база не подключена» (ожидаемо).
3. Фото яблони при выбранной яблоне → classify работает.

## Следующая сессия (4)

Контент и CV для яблони: больше статей, обучение `.pth`, метрики.
