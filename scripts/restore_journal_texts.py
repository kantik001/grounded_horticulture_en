#!/usr/bin/env python3
"""Восстановить тексты статей с journalkubansad из PDF (формат journal_ingest, до normalize)."""
from __future__ import annotations

import json
import re
import sys
import time
from pathlib import Path

import requests

_SCRIPTS = Path(__file__).resolve().parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from enrich_manual_articles import discover_manual_paths  # noqa: E402
from journal_ingest import (  # noqa: E402
    build_article_body,
    download_pdf,
    extract_pdf_text,
    norm_text,
    pdf_cache_path,
)

ROOT = _SCRIPTS.parent
DATA = ROOT / "data"
CATALOG = ROOT / "_tmp" / "journal_catalog.json"
CROPS = ("apple", "pear", "plum")


def parse_item_from_file(path: Path, crop: str) -> dict:
    text = path.read_text(encoding="utf-8", errors="ignore")
    title = ""
    m = re.search(r"-\s*Заголовок:\s*(.+?)(?=\n-\s|\n\n|$)", text, re.DOTALL | re.I)
    if m:
        title = norm_text(m.group(1).split(" - URL:")[0])
    url_m = re.search(r"https?://journalkubansad\.ru/pdf/[^\s\)]+", text)
    pdf_url = url_m.group(0).rstrip(").,") if url_m else ""
    doi_m = re.search(r"10\.30679/2219-5335-[^\s]+", text)
    authors = ""
    am = re.search(r"-\s*Авторы:\s*(.+?)(?=\n-\s|\n\n|$)", text, re.DOTALL | re.I)
    if am:
        authors = norm_text(am.group(1))
    abstract = ""
    for pat in (
        r"Кратко для садовода[^\n]*\n(.+?)(?=\n(?:Реферат|Цифры|Цель|Дисклеймер)|\Z)",
        r"Реферат \(полный\):\s*(.+?)(?=\n(?:Цифры|Цель|Дисклеймер)|\Z)",
        r"Аннотация\.\s*(.+?)(?=Ключевые слова|Введение|\Z)",
    ):
        mm = re.search(pat, text, re.DOTALL | re.I)
        if mm:
            abstract = norm_text(mm.group(1))
            if len(abstract) > 200:
                break
    return {
        "title": title or path.stem,
        "pdf_url": pdf_url,
        "doi": doi_m.group(0) if doi_m else "",
        "authors": authors,
        "abstract": abstract,
        "crop_id": crop,
    }


def catalog_by_url() -> dict[str, dict]:
    if not CATALOG.exists():
        return {}
    items = json.loads(CATALOG.read_text(encoding="utf-8"))
    return {it["pdf_url"]: it for it in items}


def is_corrupted(text: str) -> bool:
    return bool(
        re.search(
            r"Краткодля|мероприятийпо|яблонив|садоводаи|завязикак|Цельи\s|Реферат\s*\(",
            text,
        )
    )


def main() -> None:
    by_url = catalog_by_url()
    manual = set(discover_manual_paths())
    session = requests.Session()
    session.headers["User-Agent"] = "doctor_gardens_ai-kb-bot/1.0 (research; local)"
    restored = skipped = failed = 0

    for crop in CROPS:
        for path in sorted((DATA / crop).glob("article*.txt")):
            rel = f"data/{crop}/{path.name}"
            if rel in manual:
                skipped += 1
                continue
            text = path.read_text(encoding="utf-8", errors="ignore")
            if "journalkubansad.ru/pdf/" not in text:
                continue
            if "Кратко:" in text and "Новое для" in text:
                skipped += 1
                continue
            if not is_corrupted(text) and "Кратко для садовода" in text:
                skipped += 1
                continue
            corrupted = is_corrupted(text)
            item = parse_item_from_file(path, crop)
            if not item["pdf_url"]:
                skipped += 1
                continue
            cat = by_url.get(item["pdf_url"])
            if cat:
                item.update(
                    {k: norm_text(cat[k]) for k in ("title", "abstract", "doi", "authors") if cat.get(k)}
                )
                item["title"] = item["title"].split(" - Авторы:")[0].strip()
                item["crop_id"] = crop
            elif corrupted:
                item["title"] = path.stem
                item["abstract"] = ""
            if corrupted:
                item["abstract"] = ""
            else:
                item["abstract"] = item.get("abstract", "").replace("и ассистента: ", "").strip()

            pdf_path = pdf_cache_path(item["pdf_url"])
            try:
                download_pdf(item["pdf_url"], pdf_path, session)
                pdf_text = extract_pdf_text(pdf_path)
            except Exception as e:
                print(f"fail {path.name}: {e}")
                failed += 1
                continue

            body = build_article_body(item, pdf_text)
            path.write_text(body, encoding="utf-8")
            restored += 1
            print(f"ok {crop}/{path.name}")
            time.sleep(0.3)

    print(f"restored={restored} skipped={skipped} failed={failed}")


if __name__ == "__main__":
    main()
