# Сессия 3 — мультикультура

Статьи для груши и сливы добавляйте в `data/pear/`, `data/plum/` и включайте
`rag_enabled: true` в `config/crops.json`, затем:

```bash
FORCE_RAG_REINDEX=true docker compose up --build classifier
# или
python scripts/reindex_rag.py
```
