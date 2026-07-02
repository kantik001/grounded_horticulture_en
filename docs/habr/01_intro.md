# ЧЕРНОВИК — серия Habr, часть 1/7
# Скопируйте содержимое ниже `---` в редактор Habr.

---

# Grounded RAG для садовода: как я собрал ассистента по научным статьям без галлюцинаций

**Кратко:** pet-project уровня production-style — гибридный поиск по ~500 статьям, оркестрация на Go, retrieval на Python, Telegram Mini App и веб-чат. В статье — зачем это нужно, как устроен пайплайн и как я меряю качество. Код открыт.

*Серия: часть 1 из 7. Далее — hybrid search, архитектура Go/Python, eval и деплой.*

---

## Зачем вообще не «просто ChatGPT»

Садовод задаёт вопросы не из учебника по ML, а из практики:

- «Какой подвой СК-4 на юге?»
- «Какие признаки парши на листьях?»
- «Чем опасен марссониоз в влажный год?»

Универсальная LLM **знает** про паршу в общих чертах, но:

1. **Путает дозировки и региональные рекомендации** — в агро ошибка в цифре дорого стоит.
2. **Не опирается на ваш корпус** — журнальные статьи, методички, внутренние материалы.
3. **Плохо ловит доменные синонимы** — «СК-4», «СК 4», «ск 4» в одном тексте, «марссониоз» / *Marssonina*.

Мне нужен был не «умный собеседник», а **справочник с генерацией**: ответ строится из найденных фрагментов статей, а не из «памяти» модели. Это и есть **grounded RAG** (retrieval-augmented generation с привязкой к источнику).

Проект: **grounded-horticulture**. Публичный репозиторий: [github.com/kantik001/grounded-horticulture_ru](https://github.com/kantik001/grounded-horticulture_ru).

---

## Что получилось в цифрах

| Метрика | Значение |
|---------|----------|
| Статьи в базе (яблоня / груша / слива) | ~344 / ~42 / ~108 |
| Фрагментов в индексе (чанков) | **~14 554** |
| Вопросов в eval-наборе | **68** |
| Retrieval regression | **100%** на последнем прогоне |
| Unit-тесты | Go + pytest (~45), CI на GitHub Actions |

Демо в README: GIF чата и админки.

> **Честно:** CV (MobileNetV2) в бете — без обученных весов на болезнях. Акцент серии — **текстовый RAG**.

---

## Архитектура в двух словах

```
Browser / Telegram  →  Go Server  →  Python /rag/context
                            ↓
                       LLM API
                            ↓
                    verify + ответ
```

Go: сессии, auth, промпт, LLM, verify. Python: **только поиск** по статьям, без вызова LLM.

Контракт:

```json
{"question": "Какие признаки парши на листьях?", "crop_id": "apple"}
```

Ответ Python: `context`, `fragments`, `few_shot`, `category`. Подробно — в [части 2 серии](./02_architecture.md).

---

## Пайплайн от вопроса до ответа

1. `POST /message` (или stream).
2. Go → Python `POST /rag/context`.
3. Hybrid search → 8 фрагментов.
4. Промпт с `<context>` и жёсткими constraints.
5. LLM → clean → **verify чисел** → дисклеймер.
6. Лог `[RAG]`: `retrieval_ms`, `llm_ms`, `verify_pass`.

Фрагмент constraints (`server/rag_chat.go`):

```go
- НЕ ВЫДУМЫВАЙ. Если ответа нет в контексте — скажи: "В справочных материалах нет информации по вашему вопросу."
- Если в контексте есть конкретные цифры, дозировки — обязательно включи их в ответ.
```

---

## Как меряю качество

Eval JSONL — ожидания по **retrieved context**, не по формулировке LLM:

```json
{"crop_id": "apple", "question": "Какие признаки парши на листьях яблони?", "expect_contains": ["парша"], "category": "disease"}
```

```bash
python scripts/run_rag_eval.py --suite all
```

Подробно — в [части 5](./05_verify_eval.md).

---

## Быстрый старт

```bash
git clone https://github.com/kantik001/grounded-horticulture_ru.git
cd grounded-horticulture_ru
cp .env.example .env
docker compose up -d --build
```

Чат: `http://localhost/`, админка: `http://localhost/admin.html`.

---

## Планы серии

| Часть | Тема |
|-------|------|
| 2 | Go vs Python: кто за что отвечает |
| 3 | Hybrid search: Chroma + BM25 + reranker |
| 4 | Chunking, глоссарий, категории вопросов |
| 5 | Verify и eval |
| 6 | Docker, auth, метрики |
| 7 | Платформа и domain pack |

---

## Попробовать

[grounded-horticulture_ru](https://github.com/kantik001/grounded-horticulture_ru) · [AGRO_CASE_STUDY_RU](https://github.com/kantik001/grounded-horticulture_ru/blob/main/docs/AGRO_CASE_STUDY_RU.md)

---

*Справочная информация; не заменяет очный осмотр агронома.*

---

## Заметки автору

**Хабы:** Python, Go, Машинное обучение, NLP  
**Теги:** rag, llm, golang, python, nlp, docker  
**Картинки:** `docs/assets/demo-chat.gif`, схема архитектуры  
**Ссылка на часть 2:** опубликовать после этой статьи
