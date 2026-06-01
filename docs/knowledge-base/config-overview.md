# Разбор: папка `config/`

**Папка:** `config/` — JSON-конфиги без пересборки кода (частично монтируются в Docker).  
**Кто читает:** Go (`server/`), Python (`rag/`, `cv/`)

---

## Файлы

| Файл | Кто загружает | Назначение |
|------|---------------|------------|
| `crops.json` | Go + Python | Культуры, `cv_enabled`, `rag_enabled` |
| `prompts.json` | Go | System-промпты для RAG и фото по `crop_id` |
| `photo_templates.json` | Go | Статичные рекомендации по фото, если LLM недоступен |
| `cv_class_labels.json` | Python CV | Метки классов по `crop_id` (порядок = индекс при обучении) |
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
- **Go:** `normalizeCropID`, `requireCVEnabled` / `requireRAGEnabled` перед CV и RAG.

Добавление новой культуры: запись в JSON + папка `data/{crop_id}/` + при необходимости блоки в `prompts.json`, `few_shot.json`, `onboarding.json`.

Env: `CROPS_CONFIG_PATH` (в Docker: `/config/crops.json` на server и classifier).

**Перезагрузка:** Go — `SIGHUP` или `CONFIG_RELOAD_INTERVAL_SEC`; Python `rag/crops_config.py` — по mtime файла при следующем `load_crops_config()`.

---

## `cv_class_labels.json`

```json
{ "apple": ["healthy_apple", "apple_scab", ...] }
```

- **Python:** `cv/labels_config.py` → `default_class_labels_for_crop(crop_id)`; используется в `apple_classifier.py` и `train_classifier.py`.
- Env: `CV_CLASS_LABELS_PATH` (Docker: `/config/cv_class_labels.json`).

Для новой культуры с CV: добавьте массив меток и папки датасета с теми же именами.

---

## `prompts.json`

Ключ — `crop_id`, поля:

| Поле | Где используется |
|------|------------------|
| `rag_system` | system-сообщение LLM для текстового RAG |
| `rag_task_intro` | блок `<system>` в user-промпте RAG |
| `photo_system` | system для совета по фото (на русском) |
| `photo_user_intro` | вступление в user-промпт по фото |
| `photo_user_body` | тело промпта: класс, уверенность, top-3 (`fmt.Sprintf` с `%s`, `%.2f%%`, `%v`) |

Загрузка: `loadPromptCatalog()` при старте Go (`crops.go`), `promptsForCrop(cropID)` в `rag_chat.go` и `photo_recommendations.go`.

Env: `PROMPTS_CONFIG_PATH` (server: `/config/prompts.json`).

---

## `photo_templates.json`

Ключ — метка класса CV (`healthy_apple`, `apple_scab`, …) или **`default`** (с плейсхолдерами `{{PREDICTION}}`, `{{CONFIDENCE}}`).

Загрузка: `loadPhotoTemplates()` в `main.go` (`photo_templates.go`).

Env: `PHOTO_TEMPLATES_PATH` (по умолчанию `config/photo_templates.json`, в Docker: `/config/photo_templates.json`).

В `docker-compose.yml` каталог `./config` смонтирован в `/config` (server и classifier). Перезагрузка Go без рестарта: `docker compose kill -s HUP server` или `CONFIG_RELOAD_INTERVAL_SEC`.

Жёсткие правила RAG (без выдумывания, без названий статей) — в `rag_verify.go` / `rag_chat.go`, не в этом JSON.

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
| **crops / prompts / onboarding / photo_templates** | `docker compose restart server` (каталог `/config` в образе; для hot-reload без rebuild нужен volume — сейчас config в образе) |
| **few_shot / article_titles** | `restart classifier` или reindex не нужен для few_shot; для titles — **reindex** если меняли только titles |
| **Новые статьи в data/** | reindex — [data-pipeline.md](./data-pipeline.md) |

После изменения `crops.json` в dev также сбросьте кэш Python: перезапуск сервиса classifier (Python).

---

## Связь с `.env.example`

Конфиги не дублируют секреты. В `.env` только пути при необходимости:

- `CROPS_CONFIG_PATH`, `PROMPTS_CONFIG_PATH`, `ONBOARDING_CONFIG_PATH`, `PHOTO_TEMPLATES_PATH`

---

## Краткий итог

`config/` — **поведение продукта без Go/Python**: какие культуры активны, как говорит агроном, примеры для UI и few-shot. Первое место для контент-правок перед тяжёлым кодом.
