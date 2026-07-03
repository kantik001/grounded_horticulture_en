# Walkthrough: `rag/crops_config.py`

**Source file:** `rag/crops_config.py`  
**Config:** `config/crops.json`  
**Used by:** `rag/vector_store.py`, `rag/retrieval.py`, `cv/registry.py`, Go (`server/crops.go`), tests

---

## Why this file exists

Single source of truth for **crop list** (apple, pear, plum) and flags:

- `rag_enabled` — can search articles;
- `cv_enabled` — can classify photos.

One JSON — Python RAG/CV and Go API read the same thing (via their code / env `CROPS_CONFIG_PATH`).

---

## Config cache

```python
_CONFIG: Optional[Dict[str, Any]] = None
_CONFIG_MTIME: Optional[float] = None
```

`load_crops_config()` caches the parsed JSON together with the file **mtime** and **reloads automatically** when `crops.json` changes on disk — no restart needed. `reload_crops_config()` clears the cache explicitly (used in tests).

---

## Where `crops.json` is found

1. `CROPS_CONFIG_PATH` from env (in Docker: `/config/crops.json`) — used only if the file exists
2. `{root}/config/crops.json`
3. `/config/crops.json`

If none exists, falls back to the `{root}/config/crops.json` path (open will fail with a clear error).

---

## Functions

### `load_crops_config()`

Returns full JSON, for example:

```json
{
  "default_crop": "apple",
  "crops": {
    "apple": { "name_ru": "Apple", "cv_enabled": true, "rag_enabled": true },
    "pear": { "cv_enabled": false, "rag_enabled": true },
    "plum": { "cv_enabled": false, "rag_enabled": true },
    "demo_hr": { "cv_enabled": false, "rag_enabled": true, "ui_hidden": true }
  }
}
```

### `default_crop_id()`

Usually `"apple"` — used when `crop_id` is not provided.

### `normalize_crop_id(crop_id)`

- lowercases;
- empty → `default_crop`;
- unknown crop → **`ValueError`** with text “Unknown crop”.

Used **everywhere** before RAG and CV so search does not run on a nonexistent `crop_id`.

### `get_crop(crop_id)`

One crop dict after normalization — to check `rag_enabled` / `cv_enabled`.

### `list_crops()`

Simplified response for API `GET /crops` in `api/app.py` (`default_crop` + all crop entries).

### `crop_display_name(crop, crop_id)`

English display name when set (`name_en`), else Russian (`name_ru`), else `crop_id`.

---

## Product impact

| Flag | Effect |
|------|--------|
| `rag_enabled: false` | `retrieve_rag_context` returns error “article database not connected” |
| `cv_enabled: false` | `get_classifier_for_crop` raises ValueError |

Currently only **apple** has both `true`.

---

## Brief summary

`crops_config.py` — thin wrapper over `config/crops.json`: load, cache, normalize `crop_id`. No ML; domain configuration only.
