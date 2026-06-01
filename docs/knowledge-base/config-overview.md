# Разбор: папка `config/`

**Папка:** `config/` — JSON-конфиги без пересборки кода (частично монтируются в Docker).  
**Кто читает:** Go (`server/`), Python (`rag/`, `classifier/`)

---

## Файлы

| Файл | Кто загружает | Назначение |
|------|---------------|------------|
| `crops.json` | Go + Python | Культуры, `cv_enabled`, `rag_enabled` |
| `prompts.json` | Go | System-промпты для RAG и фото по `crop_id` |
| `few_shot.json` | Python `retrieval.py` | Примеры вопрос-ответ для промпта LLM |
| `onboarding.json` | Go | Чипы «примеров вопросов» в Web App |
| `article_titles.json` | Python `vector_store.py` | Красивые названия статей в metadata (опционально) |

Подробнее по коду: [rag-crops_config.md](./rag-crops_config.md), [server-admin-and-ux-api.md](./server-admin-and-ux-api.md).

---

## `crops.json`

```json
{
  "default_crop": "apple",
  "crops": {
    "apple": { "name_ru": "Яблоня", "emoji": "🍎", "cv_enabled": true, "rag_enabled": true },
    "pear": { "cv_enabled": false, "rag_enabled": false },
    "plum": { "cv_enabled": false, "rag_enabled": false }
  }
}
```

- **UI:** `GET /api/crops` → выпадающий список в `index.html`.
- **Python:** `normalize_crop_id`, фильтр Chroma, проверка CV.
- **Go:** `normalizeCropID`, создание сессии с `crop_id`.

Добавление новой культуры: запись в JSON + папка `data/{crop_id}/` + при необходимости блоки в `prompts.json`, `few_shot.json`, `onboarding.json`.

Env: `CROPS_CONFIG_PATH` (в Docker: `/config/crops.json` на server, `/app/config/crops.json` на classifier).

---

## `prompts.json`

Ключ — `crop_id`, поля:

| Поле | Где используется |
|------|------------------|
| `rag_system` | system-сообщение LLM для текстового RAG |
| `rag_task_intro` | блок `<system>` в user-промпте RAG |
| `photo_system` | system для совета по фото |
| `photo_user_intro` | вступление в user-промпт по фото |

Загрузка: `loadPromptCatalog()` при старте Go (`crops.go`), `promptsForCrop(cropID)` в `rag_chat.go` и `photo_recommendations.go`.

Env: `PROMPTS_CONFIG_PATH` (server: `/config/prompts.json`).

Жёсткие правила RAG (без выдумывания, без названий статей) — в коде `rag_chat.go`, не в этом JSON.

---

## `few_shot.json`

Структура: `crop_id` → категория → строка с примером.

Категории задаёт `classify_question()` в [rag-retrieval.md](./rag-retrieval.md): `fertilizer`, `disease`, `variety`, `general`.

Пример для яблони (`disease`): типичный тон ответа с цифрами из статей.

**Менять** при улучшении качества ответов LLM без правки Python.

---

## `onboarding.json`

Массив строк-вопросов на культуру.  
`GET /api/onboarding?crop_id=apple` → чипы в чате до первого сообщения.

Пути поиска файла: `ONBOARDING_CONFIG_PATH` или `/config/onboarding.json` (в образе server копируется `COPY config /config`).

---

## `article_titles.json`

Маппинг `имя_файла.txt` → длинное название для metadata Chroma и контекста LLM («Текст из статьи '…'»).

Если файла нет — используется имя файла как есть.  
**Не** показывается пользователю в чате как «Источник: …» (политика дисклеймера).

---

## Как применить изменения

| Сервис | Действие после правки JSON |
|--------|----------------------------|
| **crops / prompts / onboarding** | `docker compose restart server` (каталог `/config` в образе; для hot-reload без rebuild нужен volume — сейчас config в образе) |
| **few_shot / article_titles** | `restart classifier` или reindex не нужен для few_shot; для titles — **reindex** если меняли только titles |
| **Новые статьи в data/** | reindex — [data-pipeline.md](./data-pipeline.md) |

После изменения `crops.json` в dev также сбросьте кэш Python: перезапуск classifier.

---

## Связь с `.env.example`

Конфиги не дублируют секреты. В `.env` только пути при необходимости:

- `CROPS_CONFIG_PATH`, `PROMPTS_CONFIG_PATH`, `ONBOARDING_CONFIG_PATH`

---

## Краткий итог

`config/` — **поведение продукта без Go/Python**: какие культуры активны, как говорит агроном, примеры для UI и few-shot. Первое место для контент-правок перед тяжёлым кодом.
