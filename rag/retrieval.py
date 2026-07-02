"""
Article search (RAG retrieval): context for Go by crop_id.
"""

import json
import os
from typing import Any, Dict, List

from rag.crops_config import crop_display_name, get_crop, normalize_crop_id
from rag.debug_log import rag_debug
from rag.question_categories import classify_question

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
_few_shot_cache = None


def _few_shot_path() -> str:
    """Resolve path to few_shot.json."""
    env = os.environ.get("FEW_SHOT_CONFIG_PATH")
    if env and os.path.isfile(env):
        return env
    for candidate in (
        os.path.join(_PROJECT_ROOT, "config", "few_shot.json"),
        "/config/few_shot.json",
    ):
        if os.path.isfile(candidate):
            return candidate
    return os.path.join(_PROJECT_ROOT, "config", "few_shot.json")


def _load_few_shot() -> dict:
    """Load config/few_shot.json once into cache."""
    global _few_shot_cache
    if _few_shot_cache is not None:
        return _few_shot_cache
    path = _few_shot_path()
    with open(path, encoding="utf-8") as f:
        _few_shot_cache = json.load(f)
    return _few_shot_cache


def few_shot_for(crop_id: str, category: str) -> str:
    """Return few-shot example for crop and question category."""
    crop_shots = _load_few_shot().get(crop_id, {})
    return crop_shots.get(category) or crop_shots.get("general", "")


def retrieve_rag_context(user_question: str, crop_id: str = "apple") -> Dict[str, Any]:
    """Search Chroma and return context, few_shot, and fragments for Go."""
    q = (user_question or "").strip()
    empty = {
        "success": False,
        "error": "",
        "context": "",
        "few_shot": "",
        "category": "general",
        "fragments": [],
        "crop_id": crop_id,
    }
    if not q:
        empty["error"] = "Empty question"
        return empty

    try:
        crop_id = normalize_crop_id(crop_id)
    except ValueError as e:
        empty["error"] = str(e)
        return empty

    crop = get_crop(crop_id)
    if not crop.get("rag_enabled", True):
        label = crop_display_name(crop, crop_id)
        empty["error"] = (
            f"Article base for «{label}» is not connected yet. "
            "Choose another crop or try again later."
        )
        return empty

    from rag.vector_store import search

    category = classify_question(q)
    fragments = search(q, crop_id=crop_id, k=8, category=category)
    if not fragments:
        label = crop_display_name(crop, crop_id)
        empty["error"] = f"No information found in articles for crop «{label}»."
        return empty

    for f in fragments:
        rag_debug(f"[RAG:{crop_id}] source: {f.metadata.get('filename')}")

    context_parts: List[str] = []
    fr_json: List[Dict[str, str]] = []
    for frag in fragments:
        source_name = frag.metadata.get("filename", "Unknown source")
        context_parts.append(f"Text from article '{source_name}':\n{frag.page_content}")
        fr_json.append({"filename": source_name, "content": frag.page_content})

    context = "\n\n---\n\n".join(context_parts)
    few_shot = few_shot_for(crop_id, category)

    return {
        "success": True,
        "error": "",
        "context": context,
        "few_shot": few_shot,
        "category": category,
        "fragments": fr_json,
        "crop_id": crop_id,
    }
