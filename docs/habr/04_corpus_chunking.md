# ЧЕРНОВИК — серия Habr, часть 4/7

---

# Не только embeddings: chunking, глоссарий и категории вопросов в domain RAG

**Кратко:** качество retrieval начинается до модели — как режем статьи, расширяем запрос и выбираем few-shot. Разбор `chunking.py`, `agro_glossary.json`, `question_categories.json`.

*Серия grounded-horticulture. [Часть 3](./03_hybrid_search.md) — hybrid search.*

---

## Статья ≠ один embedding

Журнальная статья на 15–40 КБ текста. Если положить целиком:

- retrieval тащит **середину** без заголовка;
- в контекст LLM попадает **шум** (литература, сноски);
- лимит токенов съедается быстро.

Я режу на чанки **650 токенов**, overlap **80**, с приоритетом границ по секциям (`rag/chunking.py`):

```python
_SECTION_SEPARATORS = [
    "\n\nКратко для садовода:",
    "\n\nПрактические выводы:",
    "\n\nЦифры из текста и таблиц",
    "\n\n---\n\n",
    "\n\n", "\n", " ",
]

CHUNK_SIZE = 650
CHUNK_OVERLAP = 80
```

Блок **«Кратко для садовода»** часто содержит готовый ответ — splitter старается не резать его посередине.

Стабильный `chunk_id` для RRF:

```python
def chunk_id_for(doc: Document) -> str:
    crop = doc.metadata.get("crop_id", "")
    src = doc.metadata.get("source_file") or ""
    digest = hashlib.md5(doc.page_content.encode("utf-8")).hexdigest()[:12]
    return f"{crop}:{src}:{digest}"
```

Один чанк — одна запись в Chroma и в BM25.

---

## Query expansion через глоссарий

Пользователь пишет «марссониоз», в статье — *Marssonina* или «марсониевая болезнь». Жёсткий словарь в `config/agro_glossary.json`:

```json
{
  "марссониоз": ["marssonina", "марсониевая"],
  "ск-4": ["ск 4", "сортовой компонент 4"]
}
```

Код (`rag/query_expand.py`):

```python
def expand_query(query: str) -> str:
    q_lower = q.lower()
    extras = []
    for term, syns in glossary.items():
        if term not in q_lower:
            continue
        for syn in syns:
            if syn.lower() not in q_lower:
                extras.append(syn)
    if not extras:
        return q
    return f"{q} {' '.join(extras)}"
```

Расширение применяется **и** к vector, **и** к BM25. Это не LLM-query-rewrite — дешёво, предсказуемо, покрывается тестами.

Путь к файлу: `AGRO_GLOSSARY_PATH` (в Docker — `/config/agro_glossary.json`).

---

## Категории вопросов

`classify_question()` смотрит ключевые слова из `config/question_categories.json` (domain pack, не хардкод в Python).

Зачем категория:

1. **Few-shot** из `config/few_shot.json` — тон ответа для `disease` vs `rootstock`.
2. **Conditional rerank** — см. [часть 3](./03_hybrid_search.md).
3. **Аналитика** — поле `category` в логе `[RAG]`.

```python
def few_shot_for(crop_id: str, category: str) -> str:
    crop_shots = _load_few_shot().get(crop_id, {})
    return crop_shots.get(category) or crop_shots.get("general", "")
```

Пример few-shot для болезней — короткий ответ с признаками и мерами, без «воды».

---

## Мультикультура = несколько domain pack

`data/apple/`, `data/pear/`, `data/plum/` — отдельные каталоги. `crop_id` в API фильтрует индекс. В публичном репо — `sample_*.txt` и sandbox `demo_hr` (4 HR-политики) для проверки переносимости.

Добавление культуры:

1. Папка `data/{id}/` со статьями `.txt`
2. Запись в `config/crops.json` (`rag_enabled`, `name_ru`)
3. `make docker-reindex-apply`
4. Строки в `eval/rag_{id}_baseline.jsonl`

---

## Подготовка текстов (вне кода)

Ingest PDF → `.txt` в этом репозитории **не в git** (лицензия журнала). В публичном портфолио — только демо-файлы. Для Habr достаточно сказать: корпус готовился offline, в статьях — нормализованный plain text с секциями.

Типичные проблемы корпуса:

- битые переносы из PDF;
- таблицы, разбитые пробелами;
- дубли статей с разными именами файлов.

Скрипты `check_article_breaks.py`, `fix_article_metadata_titles.py` — в приватной ветке; идея: **качество KB = качество RAG**.

---

## Checklist перед добавлением статей

- [ ] Секция «Кратко для садовода» в начале или конце
- [ ] `crop_id` в metadata при индексации
- [ ] Синонимы в глоссарии для новых терминов
- [ ] 2–3 вопроса в eval JSONL
- [ ] Reindex + `run_rag_eval.py --suite {crop}`

---

## Дальше

[Часть 5](./05_verify_eval.md) — verify и eval.  
[Часть 7](./07_platform_domain_pack.md) — перенос на другой домен.

---

## Заметки автору

**Хабы:** Python, NLP  
**Картинки:** пример чанка статьи с выделенными секциями  
**Объём:** ~7–9 тыс. знаков
