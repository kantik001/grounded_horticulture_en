"""
Поиск по статьям (RAG retrieval): только векторный поиск и сборка контекста.
Вызов LLM выполняется в Go.
"""

from typing import Any, Dict, List

from rag.vector_store import search


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


FEW_SHOT_EXAMPLES = {
    "fertilizer": """
Пример вопроса: "Как удобрения влияют на рост сорта Триумф?"
Пример ответа: Удобрения значительно влияют на вегетативный рост сорта Триумф. При варианте 1 (норма) прирост побегов составил 748,5 см, при варианте 2 (удвоение) – 660,5 см, при варианте 3 (половинная доза) – 560,6 см. Удвоение дозы также вызвало подмерзание верхушек побегов и позднее осыпание листьев (декабрь-январь).
Источник: "Влияние удобрений на минеральное питание, рост, развитие и плодоношение яблони колонновидной".
""",
    "disease": """
Пример вопроса: "Какие признаки парши на листьях яблони?"
Пример ответа: Парша яблони проявляется в виде округлых бархатистых пятен оливково-зелёного цвета, которые позже становятся коричнево-чёрными. Пятна могут сливаться, листья деформируются и преждевременно опадают. Для лечения используют фунгициды (Хорус, Скор) весной и осенью.
Источник: "Болезни и вредители яблони".
""",
    "variety": """
Пример вопроса: "Какой сорт показал максимальную рентабельность на северном склоне?"
Пример ответа: На северном склоне максимальную рентабельность (496,0%) показал летний сорт «Ред Фри». Сорт «Мелба» на том же склоне достиг рентабельности 304,7%. Позднезимние сорта «Айдаред» и «Голден Делишес» продемонстрировали 361,0% и 376,5% соответственно.
Источник: "Эффективность выращивания яблони на террасированных склонах предгорной зоны КБР".
""",
    "general": """
Пример вопроса: "Что делать при появлении пятен на листьях?"
Пример ответа: При появлении пятен на листьях яблони необходимо определить причину (грибковое заболевание, вредитель или дефицит питания). Рекомендуется обратиться к агроному или отправить фото в наш бот для диагностики. Для профилактики проводите санитарную обрезку и обработку медьсодержащими препаратами.
Источник: "Общие рекомендации по уходу за яблоней".
""",
}


def retrieve_rag_context(user_question: str) -> Dict[str, Any]:
    """
    Выполняет поиск по статьям и возвращает данные для промпта в Go.

    Returns:
        success, error?, context, few_shot, category, fragments[{filename, content}]
    """
    q = (user_question or "").strip()
    if not q:
        return {
            "success": False,
            "error": "Пустой вопрос",
            "context": "",
            "few_shot": "",
            "category": "general",
            "fragments": [],
        }

    fragments = search(q, k=8)
    if not fragments:
        return {
            "success": False,
            "error": "Не нашёл информации в загруженных статьях.",
            "context": "",
            "few_shot": "",
            "category": "general",
            "fragments": [],
        }

    for f in fragments:
        print(f"[RAG] источник: {f.metadata.get('filename')}")

    context_parts: List[str] = []
    fr_json: List[Dict[str, str]] = []
    for frag in fragments:
        source_name = frag.metadata.get("filename", "Неизвестный источник")
        context_parts.append(f"Текст из статьи '{source_name}':\n{frag.page_content}")
        fr_json.append({"filename": source_name, "content": frag.page_content})

    context = "\n\n---\n\n".join(context_parts)
    category = classify_question(q)
    few_shot = FEW_SHOT_EXAMPLES.get(category, FEW_SHOT_EXAMPLES["general"])

    return {
        "success": True,
        "error": "",
        "context": context,
        "few_shot": few_shot,
        "category": category,
        "fragments": fr_json,
    }
