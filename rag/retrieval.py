"""
Поиск по статьям (RAG retrieval): контекст для Go с учётом crop_id.
"""

import json
import os
from typing import Any, Dict, List

from rag.crops_config import get_crop, normalize_crop_id
from rag.vector_store import search

_PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
_few_shot_cache = None


# Загружает config/few_shot.json один раз в кэш.
def _load_few_shot() -> dict:
    global _few_shot_cache
    if _few_shot_cache is not None:
        return _few_shot_cache
    path = os.path.join(_PROJECT_ROOT, "config", "few_shot.json")
    with open(path, encoding="utf-8") as f:
        _few_shot_cache = json.load(f)
    return _few_shot_cache


# Определяет тему вопроса по ключевым словам: fertilizer, disease, variety, general.
def classify_question(question: str) -> str:
    q_lower = question.lower()
    if any(
        kw in q_lower
        for kw in [
            "удобрение",
            "доза",
            "грамм",
            "литр",
            "подкормк",
            "азот",
            "фосфор",
            "калий",
        ]
    ):
        return "fertilizer"
    if any(
        kw in q_lower
        for kw in [
            "болезн",
            "парша",
            "пятна",
            "гниль",
            "ржавчин",
            "мучнист",
            "лечени",
        ]
    ):
        return "disease"
    if any(
        kw in q_lower
        for kw in [
            "сорт",
            "мелба",
            "голден",
            "триумф",
            "президент",
            "рентабельность",
            "склон",
        ]
    ):
        return "variety"
    return "general"


# Возвращает few-shot пример для культуры и категории вопроса.
def few_shot_for(crop_id: str, category: str) -> str:
    crop_shots = _load_few_shot().get(crop_id, {})
    return crop_shots.get(category) or crop_shots.get("general", "")


# Главная функция: поиск в Chroma → context, few_shot, fragments для Go.
def retrieve_rag_context(user_question: str, crop_id: str = "apple") -> Dict[str, Any]:
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
        empty["error"] = "Пустой вопрос"
        return empty

    try:
        crop_id = normalize_crop_id(crop_id)
    except ValueError as e:
        empty["error"] = str(e)
        return empty

    crop = get_crop(crop_id)
    if not crop.get("rag_enabled", True):
        empty["error"] = (
            f"База статей для «{crop.get('name_ru', crop_id)}» пока не подключена. "
            "Выберите другую культуру или вернитесь позже."
        )
        return empty

    fragments = search(q, crop_id=crop_id, k=8)
    if not fragments:
        empty["error"] = (
            f"Не нашёл информации в статьях по культуре «{crop.get('name_ru', crop_id)}»."
        )
        return empty

    for f in fragments:
        print(f"[RAG:{crop_id}] источник: {f.metadata.get('filename')}")

    context_parts: List[str] = []
    fr_json: List[Dict[str, str]] = []
    for frag in fragments:
        source_name = frag.metadata.get("filename", "Неизвестный источник")
        context_parts.append(f"Текст из статьи '{source_name}':\n{frag.page_content}")
        fr_json.append({"filename": source_name, "content": frag.page_content})

    context = "\n\n---\n\n".join(context_parts)
    category = classify_question(q)
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
