# RAG knowledge base (`data/`)

## Public repository

The full journal corpus (~500 files) is **not** in git — only:

| Directory | In repo | Purpose |
|-----------|---------|---------|
| `demo_hr/` | ✅ | Platform sandbox (HR policies) |
| `apple/sample_*.txt` | ✅ | Demo articles for quick start |
| `pear/`, `plum/` | README only | Add your own `.txt` locally |
| `apple/*.txt` (except sample) | ❌ gitignore | Place locally for full RAG |

After adding articles:

```bash
docker compose -p union_ai_apple stop classifier
docker compose -p union_ai_apple run --rm -e FORCE_RAG_REINDEX=true classifier python scripts/reindex_rag.py
docker compose -p union_ai_apple start classifier
```

Full eval (`python scripts/run_rag_eval.py --suite all`) expects the full corpus — on demo data only a subset of questions will pass.

## `.txt` file format

Each article is a separate file under `data/{crop_id}/`. Recommended structure:

```
Source metadata:
- Title: ...
- URL: ... (optional)

Main article text...
```

## Content rights

Do not publish third-party texts in a public git repo without permission.  
See [DATA_LICENSE.md](../DATA_LICENSE.md).
