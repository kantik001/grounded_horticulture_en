# ЧЕРНОВИК — серия Habr, часть 5/7

---

# Как я ловлю галлюцинации с цифрами: verify-слой и eval на 68 вопросах

**Кратко:** промпт запрещает выдумывать; verify проверяет числа; eval ловит регрессии retrieval до деплоя.

*Серия grounded-horticulture. [Часть 1](./01_intro.md) — обзор.*

---

## Два уровня защиты

| Уровень | Где | Что делает |
|---------|-----|------------|
| Промпт | Go, `ragUserPromptTpl` | «НЕ ВЫДУМЫВАЙ», «если нет в контексте — скажи» |
| Verify | Go `rag_verify.go` + Python `verifier.py` | Числа из ответа ⊆ числа из контекста |
| Eval | `scripts/run_rag_eval.py` | Подстроки в **retrieved context** |

LLM всё равно может ошибиться. Verify не идеален, но даёт **сигнал** в логах и метриках.

---

## Verify чисел

Извлечение чисел regex'ом (`server/rag_verify.go`):

```go
var reNumberWord = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)

func extractNumbersFromText(s string) []float64 { ... }
```

Логика: для каждого числа в **ответе** LLM проверить, есть ли оно (с допуском) в **контексте** фрагментов. Если нет — `verify_pass=false`, `verify_reason` в логе.

Также `cleanRAGAnswer` убирает:

- теги ``, `<answer>`;
- вводные «Давайте посмотрим», «Я думаю»;
- строки «Источник: …» (промпт запрещает, модель иногда нарушает).

Дисклеймер добавляется всегда:

```go
const ragAnswerDisclaimer = "Справочная информация из базы знаний. Не заменяет очный осмотр агронома; ..."
```

Контракт Go ↔ Python: общий fixture `tests/fixtures/rag_verify_contract.json`, тесты `verify_contract_test.go` и `test_verify_contract.py`.

Ограничения эвристики — в `docs/knowledge-base/rag-verify-limits.md` (единицы измерения, диапазоны «10–15»).

---

## Структурированный лог RAG

Каждый ответ пишет строку (`server/rag_log.go`):

```
[RAG] crop_id=apple session_id=… fragments=8 verify_pass=true
      retrieval_ms=203 llm_ms=1294 total_ms=1497
      category=disease question="…"
```

Те же поля — в Prometheus counters (`garden_rag_verify_pass_total`, `garden_rag_retrieval_ms_total`, …). В админке 👎 можно смотреть `rag` metadata из `analytics_events`.

---

## Eval: регрессия retrieval

Формат `eval/rag_apple_baseline.jsonl` — одна строка = один тест:

```json
{
  "crop_id": "apple",
  "question": "Какие признаки парши на листьях яблони?",
  "expect_contains": ["парша"],
  "category": "disease"
}
```

Варианты полей:

- `expect_contains_any` — синонимы (`ск-4`, `ск 4`);
- `expect_out_of_scope: true` — вопрос вне KB (ананас на Марсе);
- `category` — для отчётов по типам.

Прогон:

```bash
python scripts/run_rag_eval.py --suite all
python scripts/run_rag_eval.py --suite apple --fast   # без rerank
```

Сьюты: apple (45), pear (8), plum (10), demo_hr (5) = **68**.

**Важно:** eval проверяет **поиск**, не красоту текста LLM. Так отделяем баги индекса от багов промпта.

---

## CI: быстро vs полно

`.github/workflows/ci.yml` на каждый PR:

- `go test ./...`
- `pytest tests/`
- docker build трёх образов

`.github/workflows/rag-eval.yml` — **вручную** (`workflow_dispatch`): reindex + eval на CPU, до ~45 мин. На PR не гоняю — слишком долго для бесплатных runner'ов.

Компромисс: unit-тесты ловят логику verify и hybrid; eval — перед релизом или после смены `data/`.

---

## Пример отладки по eval-fail

1. Запустить один вопрос: `run_rag_eval.py --suite apple --question "…" --verbose`
2. Смотреть top fragments — нет подстроки?
3. Проверить: чанк в индексе? глоссарий? hybrid включён?
4. Добавить синоним или статью → reindex → повтор

---

## Честные ограничения

- Verify не ловит **перефразированные** факты без цифр.
- Eval не проверяет **юридическую** корректность рекомендаций по СЗР.
- Out-of-scope вопросы зависят от промпта LLM, не только от retrieval.

---

## Дальше

[Часть 6](./06_production_ops.md) — Docker, auth, метрики.  
Код: [server/rag_verify.go](https://github.com/kantik001/grounded-horticulture_ru/blob/main/server/rag_verify.go), [eval/](https://github.com/kantik001/grounded-horticulture_ru/tree/main/eval).

---

## Заметки автору

**Хабы:** Python, Go, NLP  
**Картинки:** скрин отчёта eval или админки с 👎 и rag metadata  
**Можно приложить:** 1–2 строки реального `[RAG]` лога (без секретов)
