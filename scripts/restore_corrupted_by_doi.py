#!/usr/bin/env python3
"""Восстановить повреждённые статьи (склейка слов) по DOI → PDF URL."""
from __future__ import annotations

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
CROPS = ("apple", "pear", "plum")

CORRUPT = re.compile(
    r"Краткодля|мероприятийпо|яблонив|садоводаи|завязикак|Цельи\s|"
    r"Влияниесорто|Цифрыиз|имеющихген|Краснодарскогокрая|сортоваяблон",
    re.I,
)


def doi_map() -> dict[str, str]:
    m: dict[str, str] = {}
    for crop in CROPS:
        for p in (DATA / crop).glob("article*.txt"):
            t = p.read_text(encoding="utf-8", errors="ignore")
            url_m = re.search(r"https?://journalkubansad\.ru/pdf/[^\s\)\]]+", t)
            doi_m = re.search(r"10\.30679/2219-5335-[^\s\)\]]+", t)
            if url_m and doi_m:
                m[doi_m.group(0).rstrip(").,")] = url_m.group(0).rstrip(").,")
    return m


def main() -> None:
    manual = set(discover_manual_paths())
    by_doi = doi_map()
    session = requests.Session()
    session.headers["User-Agent"] = "doctor_gardens_ai-kb-bot/1.0 (research; local)"
    ok = skip = fail = 0

    for crop in CROPS:
        for path in sorted((DATA / crop).glob("article*.txt")):
            rel = f"data/{crop}/{path.name}"
            if rel in manual:
                skip += 1
                continue
            text = path.read_text(encoding="utf-8", errors="ignore")
            if not CORRUPT.search(text):
                continue
            if "Кратко:" in text and "Новое для" in text:
                skip += 1
                continue
            doi_m = re.search(r"10\.30679/2219-5335-[^\s\)\]]+", text)
            if not doi_m:
                print(f"no doi {path.name}")
                fail += 1
                continue
            doi = doi_m.group(0).rstrip(").,")
            pdf_url = by_doi.get(doi)
            if not pdf_url:
                print(f"no url for doi {doi} ({path.name})")
                fail += 1
                continue
            item = {
                "title": path.stem,
                "pdf_url": pdf_url,
                "doi": doi,
                "authors": "",
                "abstract": "",
                "crop_id": crop,
            }
            try:
                pdf_path = pdf_cache_path(pdf_url)
                download_pdf(pdf_url, pdf_path, session)
                pdf_text = extract_pdf_text(pdf_path)
                body = build_article_body(item, pdf_text)
                path.write_text(body, encoding="utf-8")
                print(f"ok {path.name}")
                ok += 1
                time.sleep(0.25)
            except Exception as e:
                print(f"fail {path.name}: {e}")
                fail += 1

    print(f"ok={ok} skip={skip} fail={fail}")


if __name__ == "__main__":
    main()
