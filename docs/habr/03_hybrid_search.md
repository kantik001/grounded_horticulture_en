# ЧЕРНОВИК — серия Habr, часть 3/7

---

# Chroma + BM25 + cross-encoder: как я чинил recall на русских агро-статьях

**Кратко:** одного vector search мало для кодов сортов и редких терминов. Разбираю hybrid retrieval с RRF, условный reranker и настройки через env.

*Серия grounded-horticulture. [Часть 1](./01_intro.md) · [Часть 2](./02_architecture.md).*

---

## Симптом: vector-only промахивается

На eval-вопросах вроде «подвой СК-4» или «марссониоз» чистый semantic search иногда возвращал **похожие**, но **не те** фрагменты: общие слова про «подвой» без «СК-4», или «болезнь листьев» без *Marssonina*.

Причины:

1. **Редкие токены** — BM25 ловит точное вхождение лучше cosine.
2. **OCR и опечатки в PDF-текстах** — embedding сглаживает, но теряет точный код.
3. **Синонимы в запросе** — «СК 4» vs «СК-4» в статье.

Решение: **гибрид** vector + BM25, слияние через **RRF**, финальная пересортировка **cross-encoder** (не всегда).

---

## Embeddings: multilingual-e5-small

Модель `intfloat/multilingual-e5-small` требует префиксов (`rag/embeddings.py`):

```python
def _query(text: str) -> str:
    if t.lower().startswith("query:"):
        return t
    return f"query: {t}"

def _passage(text: str) -> str:
    return f"passage: {t}"
```

Без префиксов качество на русском заметно падает — типичная ловушка при переносе англоязычных туториалов.

---

## RRF: слияние двух ранжирований

```python
def rrf_merge(rankings: Iterable[Iterable[str]], k: int = RRF_K) -> List[str]:
    scores: dict[str, float] = {}
    for ranking in rankings:
        for rank, chunk_id in enumerate(ranking):
            scores[chunk_id] = scores.get(chunk_id, 0.0) + 1.0 / (k + rank + 1)
    return [cid for cid, _ in sorted(scores.items(), key=lambda item: -item[1])]
```

`RRF_K` по умолчанию 60 (`RAG_RRF_K` в env). Chunk идентифицируется стабильным `chunk_id` в metadata — одинаковый для Chroma и BM25.

---

## Функция `search()` — сердце retrieval

Упрощённая логика (`rag/vector_store.py`):

```python
def search(query: str, crop_id: str, k: int = 8, category: str | None = None):
    search_query = expand_query(query)  # глоссарий — часть 4
    fetch_k = max(k * 3, FETCH_K)  # default 16

    vector_docs = store.similarity_search(
        search_query, k=fetch_k, filter={"crop_id": crop_id},
    )
    vector_ids = [chunk_id для каждого doc]

    if hybrid_enabled():
        bm25_ids = bm25_search(search_query, crop_id, BM25_FETCH_K)
        merged_ids = rrf_merge([vector_ids, bm25_ids])
    else:
        merged_ids = vector_ids

    use_rerank = rerank_for_category(category)
    candidates = _collect_candidates(merged_ids, ...)

    if use_rerank and len(candidates) > 1:
        candidates = rerank_documents(search_query, candidates)

    return diversify_fragments(candidates, limit=k)
```

Порядок: **expand → vector → BM25 → RRF → (опционально) rerank → diversify**.

`diversify_fragments` не отдаёт 8 чанков из одной статьи — разнообразие источников в контексте.

---

## Условный reranker

`BAAI/bge-reranker-base` на CPU — +секунды на запрос. Включён **не всегда** (`rag/hybrid.py`):

```python
_DEFAULT_RERANK_CATEGORIES = frozenset(
    {"rootstock", "disease", "variety", "fertilizer", "relief"}
)

def rerank_for_category(category: Optional[str]) -> bool:
    if not rerank_enabled():
        return False
    if env_flag("RAG_RERANK_ALWAYS", "false"):
        return True
    cat = (category or "general").strip().lower()
    return cat in rerank_categories()
```

Для `general` / `irrigation` — быстрее, без rerank. Для `disease`, `rootstock` — точнее. Категория приходит из `classify_question()` по ключевым словам в `config/question_categories.json`.

Переключатели env:

- `RAG_HYBRID_ENABLED=true`
- `RAG_RERANK_ENABLED=true`
- `RAG_RERANK_CONDITIONAL=true`
- `RAG_FETCH_K=16`

Для локального демо можно `RAG_RERANK_ENABLED=false` — eval в режиме `--fast`.

---

## Индексация

При старте или `make docker-reindex-apply`:

1. `data/{crop_id}/*.txt` → split → `chunk_id`
2. Chroma persist в volume `chroma_data`
3. BM25 индексы в `bm25_db`

Один и тот же набор чанков — иначе RRF бессмысленен.

---

## Результат на eval

68 вопросов, метрика — подстрока в **retrieved context**. Hybrid + conditional rerank дали **100%** на последнем прогоне (перепроверять после смены корпуса).

Пример, где BM25 критичен — eval с `expect_contains_any`:

```json
{"question": "Какой подвой СК-4 используют для яблони на юге?",
 "expect_contains_any": ["ск-4", "ск 4", "с к-4"], "category": "rootstock"}
```

---

## Trade-offs

| Решение | Зачем | Цена |
|---------|-------|------|
| Hybrid | recall на кодах и терминах | Два индекса, reindex дольше |
| RRF вместо взвешенной суммы | не нужна калибровка score | константа K |
| Conditional rerank | latency на «простых» вопросах | сложнее отладка |
| e5-small | русский + скорость на CPU | не SOTA embedding |

---

## Дальше

[Часть 4](./04_corpus_chunking.md) — chunking и `agro_glossary.json`.  
Код: [rag/vector_store.py](https://github.com/kantik001/grounded-horticulture_ru/blob/main/rag/vector_store.py), [rag/hybrid.py](https://github.com/kantik001/grounded-horticulture_ru/blob/main/rag/hybrid.py).

---

## Заметки автору

**Хабы:** Python, Машинное обучение, NLP  
**Картинки:** схема pipeline vector→BM25→RRF→rerank  
**Можно вставить:** график latency с/без rerank (если снимете)
