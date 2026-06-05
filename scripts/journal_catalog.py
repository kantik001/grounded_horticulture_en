#!/usr/bin/env python3
"""Каталог статей journalkubansad.ru: сбор ссылок, фильтр apple/pear/plum, дедуп с data/."""
from __future__ import annotations

import json
import re
import time
from pathlib import Path
from urllib.parse import urljoin

import requests
from bs4 import BeautifulSoup

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "data"
CATALOG_PATH = ROOT / "_tmp" / "journal_catalog.json"
BASE = "https://journalkubansad.ru"

# Включение: полевое садоводство плодовых
INCLUDE_PATTERNS = [
    r"\bяблон",
    r"\bгруш",
    r"\bслив",
    r"\bабрикот",
    r"\bчерешн",
    r"\bалыч",
    r"\bкосточков",
    r"\bподво[йи]",
    r"\bинтенсивн.*сад",
    r"\bсадовод",
    r"\bплодонос",
    r"\bмарссони",
    r"\bпарш",
    r"\bплодожорк",
    r"\bшарк[аи]",
    r"\bppv\b",
    r"\bкбр\b",
    r"\bкабардин",
    r"\bтеррас",
    r"\bсклон",
    r"\bпредгор",
]

# Исключение: вино, декор, чистая переработка без сада
EXCLUDE_PATTERNS = [
    r"\bвиноград",
    r"\bвиноматериал",
    r"\bвинный",
    r"\bвинодел",
    r"\bдистиллят",
    r"\bконьяк",
    r"\bигрист",
    r"\bгибискус",
    r"\bдекоратив",
    r"\bландшафт",
    r"\bозеленен",
    r"\bhibiscus",
    r"\bвыжимок винограда",
    r"\bплодоношение hibiscus",
]

SECTION_BOOST = {
    "яблон": "apple",
    "груш": "pear",
    "слив": "plum",
    "абрикот": "plum",
    "черешн": "plum",
    "алыч": "plum",
    "косточков": "plum",
}


def existing_pdf_urls() -> set[str]:
    urls = set()
    for p in DATA.rglob("article*.txt"):
        text = p.read_text(encoding="utf-8", errors="ignore")
        for m in re.finditer(r"https?://journalkubansad\.ru/pdf/[^\s\]]+", text):
            urls.add(m.group(0).rstrip(").,"))
    return urls


def classify_crop(title: str, abstract: str) -> str | None:
    blob = (title + " " + abstract).lower()
    for pat in EXCLUDE_PATTERNS:
        if re.search(pat, blob, re.I):
            return None
    hits = []
    for kw, crop in SECTION_BOOST.items():
        if kw in blob:
            hits.append(crop)
    if not hits and not any(re.search(p, blob, re.I) for p in INCLUDE_PATTERNS):
        return None
    if not hits:
        if "яблон" in blob:
            return "apple"
        if "груш" in blob:
            return "pear"
        if any(x in blob for x in ("слив", "абрикот", "черешн", "алыч", "косточков")):
            return "plum"
        return None
    return max(set(hits), key=hits.count)


def fetch_page(url: str, session: requests.Session) -> str:
    r = session.get(url, timeout=60)
    r.raise_for_status()
    r.encoding = r.apparent_encoding or "utf-8"
    return r.text


def parse_listing(html: str) -> list[dict]:
    soup = BeautifulSoup(html, "html.parser")
    items = []
    for block in soup.select("div.item"):
        ref = block.select_one("div.reference")
        pdf_url = ""
        if ref:
            m = re.search(
                r"URL:\s*(https?://journalkubansad\.ru/pdf/[^\s\)]+)",
                ref.get_text(" ", strip=True),
                re.I,
            )
            if m:
                pdf_url = m.group(1).rstrip(").,")
        if not pdf_url:
            for a in block.find_all("a", href=True):
                if re.search(r"/pdf/.*\.pdf", a["href"], re.I):
                    pdf_url = urljoin(BASE, a["href"])
                    break
        if not pdf_url:
            continue
        title_el = block.select_one(".item_title h3 a") or block.select_one(".item_title h3")
        title = title_el.get_text(" ", strip=True) if title_el else ""
        if len(title) < 12:
            continue
        authors = []
        for a in block.select(".item_author a"):
            t = a.get_text(strip=True)
            if t:
                authors.append(t)
        abstract = ""
        ref_p = block.select_one("p.ref_text")
        if ref_p:
            abstract = ref_p.get_text(" ", strip=True)[:4000]
        doi_m = re.search(r"10\.30679/2219-5335-[^\s]+", block.get_text(" ", strip=True))
        items.append({
            "title": title[:300],
            "abstract": abstract,
            "pdf_url": pdf_url,
            "doi": doi_m.group(0) if doi_m else "",
            "authors": ", ".join(authors[:8]),
        })
    # dedupe by pdf
    seen = set()
    out = []
    for it in items:
        if it["pdf_url"] in seen:
            continue
        seen.add(it["pdf_url"])
        out.append(it)
    return out


def discover(max_pages: int = 15, start_page: int = 1) -> list[dict]:
    session = requests.Session()
    session.headers["User-Agent"] = "doctor_gardens_ai-kb-bot/1.0 (research; contact: local)"
    have = existing_pdf_urls()
    candidates = []

    for page in range(start_page, start_page + max_pages):
        url = f"{BASE}/div/all/" if page == 1 else f"{BASE}/div/all/?p=p{page}"
        try:
            html = fetch_page(url, session)
        except Exception as e:
            print(f"skip page {page}: {e}")
            continue
        batch = parse_listing(html)
        print(f"page {page}: blocks {len(batch)}")
        for it in batch:
            if it["pdf_url"] in have:
                continue
            crop = classify_crop(it["title"], it["abstract"])
            if not crop:
                continue
            it["crop_id"] = crop
            candidates.append(it)
        time.sleep(0.8)

    # dedupe candidates
    by_pdf = {c["pdf_url"]: c for c in candidates}
    return list(by_pdf.values())


def main() -> None:
    import argparse

    p = argparse.ArgumentParser()
    p.add_argument("--pages", type=int, default=333, help="сколько страниц каталога (всего ~333)")
    p.add_argument("--start", type=int, default=1)
    args = p.parse_args()
    items = discover(max_pages=args.pages, start_page=args.start)
    CATALOG_PATH.parent.mkdir(parents=True, exist_ok=True)
    CATALOG_PATH.write_text(json.dumps(items, ensure_ascii=False, indent=2), encoding="utf-8")
    by_crop = {}
    for it in items:
        by_crop.setdefault(it["crop_id"], []).append(it)
    print(f"catalog: {len(items)} new candidates -> {CATALOG_PATH}")
    for c, lst in sorted(by_crop.items()):
        print(f"  {c}: {len(lst)}")


if __name__ == "__main__":
    main()
