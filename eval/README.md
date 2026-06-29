# RAG eval — регрессии качества по домену

Универсальный механизм платформы: один формат для агро, `demo_hr` и будущих клиентов.

## Файлы

| Файл | Домен (`crop_id`) | Вопросов |
|------|-------------------|----------|
| `rag_apple_baseline.jsonl` | `apple` | 45 |
| `rag_pear_baseline.jsonl` | `pear` | 8 |
| `rag_plum_baseline.jsonl` | `plum` | 10 |
| `rag_demo_hr_baseline.jsonl` | `demo_hr` | 5 |

> Наборы синхронизированы с академической базой статей (журнал «Плодоводство и
> виноградарство Юга России»): подвои, склоны/террасы КБР, питание, защита
> (марссониоз, плодожорка). Старые вопросы под удалённые микро-статьи (мучнистая
> роса, бактериальный ожог, ржавчина, тля и т.п.) убраны — их нет в текущей KB.

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

- `expect_contains` — подстроки в **контексте** retrieval (режим по умолчанию) или в ответе LLM (`--full`). Скрипт допускает русскую морфологию (стем: «подвой» ↔ «подвои»).
- `expect_contains_any` — достаточно одной подстроки из списка (синонимы: марссониоз / Marssonina).
- `expect_out_of_scope: true` — вопрос вне KB; ожидаем пустой/слабый контекст или фразу «нет в материалах» в full-режиме.

## Запуск

```bash
# Retrieval-only (Python POST /rag/context)
python scripts/run_rag_eval.py --suite apple
python scripts/run_rag_eval.py --suite pear
python scripts/run_rag_eval.py --suite plum
python scripts/run_rag_eval.py --suite demo_hr
python scripts/run_rag_eval.py --suite all

make eval-retrieval
```

Требуется доступный `CLASSIFIER_RAG_URL` (по умолчанию `http://localhost:5000/rag/context`).

Результаты: `eval/results/<timestamp>_<suite>.json`.

Portfolio summary (EN): [`docs/AGRO_CASE_STUDY_EN.md`](../docs/AGRO_CASE_STUDY_EN.md).

## Когда гонять

- После `reindex_rag.py` / admin reindex (пересобираются Chroma **и** BM25). В Docker: `make docker-reindex-apply` или reindex + `docker compose restart classifier` (см. [data-pipeline.md](../docs/knowledge-base/data-pipeline.md)).
- После правок `data/`, `prompts.json`, `few_shot.json`.
- Перед пилотом и перед релизом.

См. [../docs/knowledge-base/quality-eval-and-rag-logs.md](../docs/knowledge-base/quality-eval-and-rag-logs.md).
