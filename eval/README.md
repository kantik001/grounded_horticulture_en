# RAG eval — регрессии качества по домену

Универсальный механизм платформы: один формат для агро, `demo_hr` и будущих клиентов.

## Файлы

| Файл | Домен (`crop_id`) | Вопросов |
|------|-------------------|----------|
| `rag_apple_baseline.jsonl` | `apple` | 30 |
| `rag_demo_hr_baseline.jsonl` | `demo_hr` | 5 |

Формат строки JSON:

```json
{
  "crop_id": "apple",
  "question": "Какие признаки парши?",
  "expect_contains": ["парша", "пятн"],
  "expect_context": true,
  "expect_out_of_scope": false,
  "category": "disease"
}
```

- `expect_contains` — подстроки в **контексте** retrieval (режим по умолчанию) или в ответе LLM (`--full`).
- `expect_out_of_scope: true` — вопрос вне KB; ожидаем пустой/слабый контекст или фразу «нет в материалах» в full-режиме.

## Запуск

```bash
# Retrieval-only (Python POST /rag/context)
python scripts/run_rag_eval.py --suite apple
python scripts/run_rag_eval.py --suite demo_hr
python scripts/run_rag_eval.py --suite all

make eval-retrieval
```

Требуется доступный `CLASSIFIER_RAG_URL` (по умолчанию `http://localhost:5000/rag/context`).

Результаты: `eval/results/<timestamp>_<suite>.json`.

## Когда гонять

- После `reindex_rag.py` / admin reindex. В Docker после reindex: `docker compose restart classifier` (см. [data-pipeline.md](../docs/knowledge-base/data-pipeline.md)).
- После правок `data/`, `prompts.json`, `few_shot.json`.
- Перед пилотом и перед релизом.

См. [../docs/knowledge-base/quality-eval-and-rag-logs.md](../docs/knowledge-base/quality-eval-and-rag-logs.md).
