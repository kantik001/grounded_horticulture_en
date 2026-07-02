# ЧЕРНОВИК — серия Habr, часть 2/7

---

# Разделение Go и Python в RAG-пайплайне: кто собирает ответ, а кто только ищет в статьях

**Кратко:** Python возвращает фрагменты статей; Go вызывает LLM, верифицирует ответ и отдаёт его клиенту. Разбираю контракт `/rag/context` и почему не «всё в LangChain».

*Серия grounded-horticulture. [Часть 1](./01_intro.md) — вводная.*

---

## Проблема монолита на Python

Типичный tutorial: один FastAPI-сервис — embeddings, retrieval, prompt, LLM, ответ. Для демо работает. Для продукта с Telegram, сессиями и rate limit быстро упираешься в:

- **два типа клиентов** (Web App + браузер с API-ключом);
- **историю диалога** в Postgres;
- **стриминг** SSE;
- **единый контракт ошибок** для UI.

Я вынес «продуктовую» логику в **Go (Gin)**, а Python оставил **ML-сервисом** с двумя эндпоинтами: `/classify` (CV) и `/rag/context` (retrieval).

---

## Контракт `/rag/context`

**Запрос** (Go → Python):

```json
{
  "question": "Какой подвой СК-4 на юге?",
  "crop_id": "apple"
}
```

**Ответ** при успехе:

```json
{
  "success": true,
  "context": "…склеенный текст фрагментов…",
  "few_shot": "…пример тона ответа…",
  "category": "rootstock",
  "fragments": [
    {"filename": "Подвои СК", "content": "…"}
  ],
  "crop_id": "apple"
}
```

При отсутствии контекста — HTTP 422, `success: false` (штатный сценарий, не 500).

Go **не парсит** Chroma и не знает про BM25. Python **не знает** API-ключ OpenRouter. Граница жёсткая — проще тестировать и менять retrieval без redeploy Go.

---

## Код на стороне Go

Типы и HTTP-клиент (`server/rag_chat.go`):

```go
type RAGFragment struct {
    Filename string `json:"filename"`
    Content  string `json:"content"`
}

type pythonRAGContextResponse struct {
    Success   bool          `json:"success"`
    Error     string        `json:"error,omitempty"`
    Context   string        `json:"context,omitempty"`
    FewShot   string        `json:"few_shot,omitempty"`
    Category  string        `json:"category,omitempty"`
    Fragments []RAGFragment `json:"fragments,omitempty"`
}

func fetchRAGContext(question, cropID string) (*pythonRAGContextResponse, error) {
    body := map[string]string{"question": question, "crop_id": cropID}
    // POST config.PythonRAGURL, timeout 120s
}
```

Важная деталь: **422 от Python** при `success: false` Go обрабатывает как «мягкий отказ» (нет статей по вопросу), а не как падение сети.

Сборка промпта — тоже Go:

```go
func buildRAGUserPrompt(question, context, fewShot, taskIntro string) string {
    return fmt.Sprintf(ragUserPromptTpl, taskIntro, context, fewShot, question)
}
```

Шаблон `ragUserPromptTpl` содержит `<system>`, `<context>`, `<examples>`, `<constraints>`. Системный промпт по домену читается из `config/prompts.json` — без пересборки бинарника (`promptsForCrop`).

---

## Код на стороне Python

Тонкий Flask-handler (`api/app.py`):

```python
@app.route("/rag/context", methods=["POST"])
def rag_context():
    data = request.get_json(silent=True) or {}
    question = (data.get("question") or "").strip()
    crop_id = data.get("crop_id") or "apple"
    payload = retrieve_rag_context(question, crop_id=crop_id)
    status = 200 if payload.get("success") else 422
    return jsonify(payload), status
```

Ядро — `retrieve_rag_context` в `rag/retrieval.py`:

```python
def retrieve_rag_context(user_question: str, crop_id: str = "apple") -> Dict[str, Any]:
    category = classify_question(q)
    fragments = search(q, crop_id=crop_id, k=8, category=category)
    # context = склейка fragments, few_shot из config/few_shot.json
```

`crop_id` — идентификатор **domain pack** (яблоня, груша, `demo_hr`). Фильтр в Chroma: `filter={"crop_id": crop_id}`.

---

## Поток `/message` с сессией

```
Client → POST /api/message {session_id, text, crop_id}
       → Go: history из Postgres
       → fetchRAGContext
       → messages[] для LLM (system + history + user prompt)
       → callLLMCompletion / stream
       → finalizeRAGAnswer (clean + verify + disclaimer)
       → save message + logRAGTrace
```

Устаревший `POST /chat` без сессии помечен `Deprecation: true` — UI использует `/message`.

---

## Trade-offs

| Плюс | Минус |
|------|-------|
| Независимый eval retrieval | Два деплоя, два healthcheck |
| Go — привычный слой для auth/DB | +1 сетевой hop (~200 ms retrieval) |
| Python можно перезапустить после reindex | Нужен согласованный контракт JSON |

Альтернатива «всё в Python» имела бы смысл, если бы продукт был только Jupyter + Gradio. Для Telegram + админки + metrics Go окупился.

---

## Что дальше

- [Часть 3](./03_hybrid_search.md) — что происходит внутри `search()`.
- [Часть 5](./05_verify_eval.md) — verify после LLM.

Код: [server/rag_chat.go](https://github.com/kantik001/grounded-horticulture_ru/blob/main/server/rag_chat.go), [rag/retrieval.py](https://github.com/kantik001/grounded-horticulture_ru/blob/main/rag/retrieval.py).

---

## Заметки автору

**Хабы:** Go, Python, NLP  
**Картинки:** диаграмма последовательности Client → Go → Python → LLM  
**Объём:** ~8–10 тыс. знаков
