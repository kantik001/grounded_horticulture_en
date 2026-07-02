# ЧЕРНОВИК — серия Habr, часть 7/7

---

# От агробота к платформе: domain pack и sandbox demo_hr

**Кратко:** проект задуман как клонируемая платформа — ядро + заменяемый пакет данных и конфигов. Показываю на `demo_hr` и правиле «не хардкодить домен в core».

*Серия grounded-horticulture, финал. [Часть 1](./01_intro.md) — вводная.*

---

## Зачем платформа, если первый продукт — сад

На собеседовании часто спрашивают: «это только для яблонь?» Ответ: **первый domain pack — агро**, каркас — универсальный grounded-ассистент по `.txt` базе знаний.

Цель архитектуры (`docs/ARCHITECTURE.md`):

```
┌─────────────────────────────────────────┐
│  Platform core (Go, Python RAG, CI)     │
└─────────────────┬───────────────────────┘
                  │
     ┌────────────┼────────────┐
     ▼            ▼            ▼
  agro/apple   demo_hr    будущий клиент
  data+config  data+config
```

Клонирование = скопировать репо, заменить domain pack, переиндексировать, подставить branding.

---

## Три слоя

| Слой | Что входит | Меняется? |
|------|------------|-----------|
| **Core** | `server/`, `api/`, `rag/`, `migrations/`, Docker, eval-механизм | Нет |
| **Domain pack** | `data/{id}/`, `crops.json`, `prompts.json`, `few_shot.json`, `question_categories.json`, `onboarding.json`, `branding.json` | **Да** |
| **Optional** | `cv/`, `photo_templates.json`, Telegram webapp | По задаче |

`crop_id` в API — по сути **`domain_id`** (историческое имя для агро).

---

## Sandbox demo_hr

В `data/demo_hr/` — четыре файла HR-политик (отпуск, больничный, удалёнка, кодекс). В `config/crops.json`:

```json
{
  "id": "demo_hr",
  "name_ru": "HR (демо)",
  "rag_enabled": true,
  "cv_enabled": false
}
```

Eval: `eval/rag_demo_hr_baseline.jsonl` — 5 вопросов, те же правила `expect_contains`.

Зачем:

- доказать, что **retrieval не привязан** к «парше» и «подвою»;
- быстрый smoke для работодателя из HR/IT без агро-контекста;
- проверка `classify_question` на нейтральных категориях.

Запуск:

```bash
python scripts/run_rag_eval.py --suite demo_hr
```

В UI выбрать культуру «HR (демо)» — те же чат и админка.

---

## Что в domain pack, а что нельзя тащить в core

**Правильно в config/data:**

- ключевые слова категорий (`question_categories.json`);
- синонимы (`agro_glossary.json` → для HR будет `hr_glossary.json` по тому же механизму);
- промпты и few-shot;
- чипы onboarding;
- дисклеймер в `branding.json`.

**Неправильно в `rag/*.py`:**

- списки болезней яблони;
- захардкоженные названия сортов.

Рефакторинг P1 как раз вынес категории из кода в JSON.

---

## Минимальный чеклист нового домена

1. `data/my_domain/*.txt`
2. Запись в `config/crops.json`
3. Секции в `prompts.json`, `few_shot.json`, `onboarding.json`
4. `branding.json` — заголовок, дисклеймер
5. `eval/rag_my_domain_baseline.jsonl` — хотя бы 5 вопросов
6. `make docker-reindex-apply`
7. `run_rag_eval.py --suite my_domain`

CV отключить: `"cv_enabled": false`.

---

## Публичный vs приватный репозиторий

[grounded-horticulture_ru](https://github.com/kantik001/grounded-horticulture_ru) — **код + demo data**. Полный журнальный корпус (~500 статей) в git не входит ([DATA_LICENSE.md](https://github.com/kantik001/grounded-horticulture_ru/blob/main/DATA_LICENSE.md)).

Для Habr и резюме:

- показываем **инженерию** (hybrid RAG, eval, verify);
- честно говорим про масштаб корпуса в case study;
- demo достаточно, чтобы поднять стек локально.

Публикация через orphan-коммит — без утечки старых blob'ов корпуса в git history.

---

## Roadmap без маркетинга

Из `docs/ROADMAP.md` — реалистично для пост-серии:

- обучение CV на реальном датасете болезней;
- OCR-починка битых PDF-текстов;
- schema_migrations для Postgres;
- EN-репозиторий и статья на dev.to (зеркало RU-серии).

Не обещать SaaS и мульти-тенант в ближайшем посте — это другой порядок задач.

---

## Итог серии

| Часть | Главная мысль |
|-------|----------------|
| 1 | Зачем grounded RAG и цифры |
| 2 | Go оркестрирует, Python ищет |
| 3 | Hybrid search |
| 4 | Корпус и конфиги |
| 5 | Verify + eval |
| 6 | Деплой и war stories |
| 7 | Платформа и переносимость |

Спасибо за чтение. Репозиторий открыт — велком issues и PR.

---

## Заметки автору

**Хабы:** Архитектура, Python, Go  
**Тон:** финал серии, короче предыдущих (~6–8 тыс. знаков)  
**Ссылка:** на EN repo / dev.to, когда будет готов
