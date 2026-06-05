#!/usr/bin/env python3
"""Аудит data/plum/: статьи, где основная тема — не слива."""

from __future__ import annotations

import json
import re
from collections import Counter
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLUM_DIR = ROOT / "data" / "plum"
OUT_MD = ROOT / "docs" / "knowledge-base" / "plum_miscategorized_audit.md"
OUT_JSON = ROOT / "eval" / "plum_miscategorized_audit.json"

# Ручные статьи с plum в имени — не помечать misc без сильного сигнала.
MANUAL_PLUM_PREFIX = re.compile(
    r"^article\d+_plum_|^article\d+_(stanley|polimiks|renklod)",
    re.I,
)

FN_CHERRY_HINT = re.compile(r"chereshn|cherry|vishn", re.I)
FN_PLUM_HINT = re.compile(
    r"_plum_|slivy|sharki|stanley|renklod|polimiks|stonefruit|fitaktiv|ppv",
    re.I,
)
FN_OTHER_CROP_HINT = re.compile(
    r"tomat|grush|pyrus|zemlyanik|klubnik|abrikos|apricot|persik|peach",
    re.I,
)

CROP_PATTERNS: dict[str, list[str]] = {
    "plum": [
        r"слив[аыоеуюйём]",
        r"шарк[аи]",
        r"станлей",
        r"ренклод",
        r"алыч[аи]",
        r"ppv",
        r"plum",
    ],
    "cherry": [r"черешн", r"вишн[яи]", r"cherry"],
    "apricot": [r"абрикос"],
    "peach": [r"персик"],
    "pear": [r"груш[аы]", r"pyrus"],
    "tomato": [r"томат", r"лико пер", r"solanum", r"т-34"],
    "strawberry": [r"земляник", r"клубник", r"fragaria"],
}

# «сливовидн*» у томатов — не считать за сливу.
TOMATO_PLUM_SHAPE = re.compile(r"сливовидн", re.I)

TITLE_RE = re.compile(r"^- Заголовок:\s*(.+)$", re.M)
RAG_CULTURE_RE = re.compile(r"^- Культура \(RAG\):\s*(\S+)", re.M)


@dataclass
class ArticleAudit:
    file: str
    title: str
    rag_crop: str
    scores: dict[str, int]
    primary_crop: str
    verdict: str  # ok | mixed | misc | review
    reason: str
    suggested_action: str


def _read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8", errors="replace")


def _title(text: str, filename: str) -> str:
    m = TITLE_RE.search(text)
    if m:
        return m.group(1).strip()[:120]
    return filename


def _rag_crop(text: str) -> str:
    m = RAG_CULTURE_RE.search(text)
    return m.group(1).strip() if m else "?"


def _count_crop(text: str) -> dict[str, int]:
    lower = text.lower()
    scores: dict[str, int] = {}
    for crop, patterns in CROP_PATTERNS.items():
        n = 0
        for pat in patterns:
            n += len(re.findall(pat, lower, flags=re.I))
        if crop == "plum":
            n -= len(TOMATO_PLUM_SHAPE.findall(lower))
            n = max(0, n)
        scores[crop] = n
    return scores


def _primary(scores: dict[str, int]) -> str:
    if not scores or max(scores.values()) == 0:
        return "unknown"
    return max(scores.items(), key=lambda x: x[1])[0]


def classify(path: Path) -> ArticleAudit:
    text = _read_text(path)
    filename = path.name
    title = _title(text, filename)
    rag = _rag_crop(text)
    scores = _count_crop(text)
    fn_lower = filename.lower()
    if FN_PLUM_HINT.search(filename):
        scores["plum"] = scores.get("plum", 0) + 20
    primary = _primary(scores)
    plum_score = scores.get("plum", 0)
    primary_score = scores.get(primary, 0)
    manual_plum = bool(MANUAL_PLUM_PREFIX.search(filename))

    fn_hint = None
    if FN_CHERRY_HINT.search(filename) and not FN_PLUM_HINT.search(filename):
        fn_hint = "cherry"
    elif FN_OTHER_CROP_HINT.search(filename) and not FN_PLUM_HINT.search(filename):
        if "tomat" in fn_lower:
            fn_hint = "tomato"
        elif "grush" in fn_lower or "pyrus" in fn_lower:
            fn_hint = "pear"
        elif "zemlyanik" in fn_lower or "klubnik" in fn_lower:
            fn_hint = "strawberry"
        elif "abrikos" in fn_lower or "apricot" in fn_lower:
            fn_hint = "apricot"
        elif "persik" in fn_lower or "peach" in fn_lower:
            fn_hint = "peach"

    verdict = "ok"
    reason = "Доминирует слива или сбалансированный косточковый контекст со сливой."
    action = "оставить"

    stone_total = sum(scores.get(c, 0) for c in ("plum", "cherry", "apricot", "peach"))
    non_plum_stone = stone_total - plum_score

    if primary in ("tomato", "strawberry", "pear"):
        verdict = "misc"
        reason = f"Основная тема: {primary} (score {primary_score}, слива {plum_score})."
        action = f"исключить из plum RAG или перенести в data/{_crop_dir(primary)}/"
    elif fn_hint and fn_hint != "plum" and plum_score < primary_score:
        verdict = "misc"
        reason = f"Имя файла указывает на {fn_hint}; primary={primary}."
        action = f"перенести/исключить; проверить ingest"
    elif FN_PLUM_HINT.search(filename) and plum_score >= 8:
        verdict = "ok"
        reason = "Имя файла и текст указывают на сливу."
        action = "оставить"
    elif primary == "cherry" and plum_score < primary_score * 0.5 and primary_score >= 8:
        verdict = "misc"
        reason = f"Преобладает черешня/вишня ({primary_score} vs слива {plum_score})."
        action = "исключить из plum или тег ui_hidden; опционально data/cherry/"
    elif primary == "apricot" and plum_score < 3 and primary_score >= 6:
        verdict = "misc"
        reason = f"Преобладает абрикос ({primary_score} vs слива {plum_score})."
        action = "mixed article19-style — пометить multi или исключить"
    elif primary == "peach" and plum_score < 3 and primary_score >= 6:
        verdict = "misc"
        reason = f"Преобладает персик ({primary_score} vs слива {plum_score})."
        action = "исключить из plum или multi-косточковые"
    elif manual_plum and plum_score >= 3:
        verdict = "ok"
        reason = "Ручная статья про сливу (имя файла + текст)."
        action = "оставить"
    elif plum_score == 0 and primary_score >= 4:
        verdict = "misc"
        reason = f"Нет упоминаний сливы; primary={primary} ({primary_score})."
        action = "исключить из индекса plum"
    elif plum_score > 0 and non_plum_stone > plum_score * 1.5 and primary != "plum":
        verdict = "mixed"
        reason = (
            f"Косточковые mixed: слива {plum_score}, "
            f"другие {primary} {primary_score}."
        )
        action = "оставить с осторожностью или добавить фильтр по теме"
    elif plum_score < 5 and primary_score >= 12 and primary != "plum":
        verdict = "review"
        reason = f"Слабый сигнал сливы ({plum_score}), сильный {primary} ({primary_score})."
        action = "ручная проверка заголовка/PDF"

    return ArticleAudit(
        file=filename,
        title=title,
        rag_crop=rag,
        scores=scores,
        primary_crop=primary,
        verdict=verdict,
        reason=reason,
        suggested_action=action,
    )


def _crop_dir(crop: str) -> str:
    return {
        "tomato": "— (нет в crops.json)",
        "strawberry": "— (нет в crops.json)",
        "pear": "pear",
        "cherry": "— (нет в crops.json)",
        "apricot": "— (multi stonefruit)",
        "peach": "— (multi stonefruit)",
    }.get(crop, crop)


def main() -> int:
    paths = sorted(PLUM_DIR.glob("article*.txt"))
    audits = [classify(p) for p in paths]

    by_verdict: dict[str, list[ArticleAudit]] = {}
    for a in audits:
        by_verdict.setdefault(a.verdict, []).append(a)

    summary = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "total": len(audits),
        "ok": len(by_verdict.get("ok", [])),
        "mixed": len(by_verdict.get("mixed", [])),
        "misc": len(by_verdict.get("misc", [])),
        "review": len(by_verdict.get("review", [])),
        "articles": [asdict(a) for a in audits],
    }

    OUT_JSON.parent.mkdir(parents=True, exist_ok=True)
    OUT_MD.parent.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    misc_by_crop = Counter(a.primary_crop for a in by_verdict.get("misc", []))

    lines = [
        "# Аудит miscategorized: `data/plum/`",
        "",
        f"Сгенерировано: `{summary['generated_at']}` — `python scripts/audit_plum_miscategorized.py`",
        "",
        "## Сводка",
        "",
        f"| Всего статей | OK | Mixed | Misc | На проверку |",
        f"|-------------:|---:|------:|-----:|------------:|",
        f"| {summary['total']} | {summary['ok']} | {summary['mixed']} | {summary['misc']} | {summary['review']} |",
        "",
        "- **OK** — слива доминирует или ручная статья про сливу.",
        "- **Mixed** — косточковые смешанные (слива + черешня/абрикос); шум для узких вопросов.",
        "- **Misc** — основная тема другая культура (томат, груша, черешня…).",
        "- **Review** — пограничные случаи, нужна ручная проверка PDF.",
        "",
        "### Misc по основной культуре",
        "",
        "| Primary | Статей |",
        "|---------|-------:|",
    ]
    for crop, n in misc_by_crop.most_common():
        lines.append(f"| {crop} | {n} |")
    lines.extend(["", ""])

    for verdict, label in [
        ("misc", "Miscategorized — рекомендуется убрать из plum RAG"),
        ("review", "На ручную проверку"),
        ("mixed", "Mixed stonefruit — оставить или пометить"),
    ]:
        items = sorted(by_verdict.get(verdict, []), key=lambda x: x.file)
        if not items:
            continue
        lines.append(f"## {label} ({len(items)})")
        lines.append("")
        lines.append("| Файл | Primary | Слива | Причина | Действие |")
        lines.append("|------|---------|------:|---------|----------|")
        for a in items:
            ps = a.scores.get("plum", 0)
            lines.append(
                f"| `{a.file}` | {a.primary_crop} | {ps} | {a.reason} | {a.suggested_action} |"
            )
        lines.append("")

    OUT_MD.write_text("\n".join(lines), encoding="utf-8")

    print(f"Всего: {summary['total']}")
    print(f"  ok={summary['ok']} mixed={summary['mixed']} misc={summary['misc']} review={summary['review']}")
    print(f"Markdown: {OUT_MD}")
    print(f"JSON: {OUT_JSON}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
