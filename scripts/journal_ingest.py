#!/usr/bin/env python3
"""Загрузка отобранных PDF с journalkubansad.ru → глубокие .txt в data/{crop}/."""
from __future__ import annotations

import argparse
import json
import re
import sys
import time
from pathlib import Path

import requests

_SCRIPTS = Path(__file__).resolve().parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from journal_catalog import (
    BASE,
    CATALOG_PATH,
    DATA,
    ROOT,
    classify_crop,
    discover,
    existing_pdf_urls,
)
from pypdf import PdfReader

PDF_DIR = ROOT / "_tmp" / "journal_pdfs"
TITLES_PATH = ROOT / "config" / "article_titles.json"


def pdf_cache_path(url: str) -> Path:
    m = re.search(r"journalkubansad\.ru/pdf/(.+)$", url)
    rel = m.group(1) if m else url.split("/")[-1]
    return PDF_DIR / rel.replace("/", "_")

UNIT_RE = re.compile(
    r"(т/га|кг/га|г/л|л/га|мм|см|м\b|%|°C|балл|ц/га|тыс\.|г/дерев)",
    re.I,
)
NUM_LINE_RE = re.compile(r"\d+[\d\s,\.]*\d*")


def next_article_num(crop: str) -> int:
    crop_dir = DATA / crop
    mx = 0
    for p in crop_dir.glob("article*.txt"):
        m = re.search(r"article(\d+)", p.name)
        if m:
            mx = max(mx, int(m.group(1)))
    return mx + 1


def slug_from_pdf(pdf_url: str) -> str:
    parts = pdf_url.rstrip("/").split("/")
    name = parts[-1].replace(".pdf", "")
    return re.sub(r"[^a-z0-9]+", "_", name.lower())[:40].strip("_") or "journal"


def clean_pdf_text(raw: str) -> str:
    t = raw.replace("\r", "\n")
    t = re.sub(r"-\s*\n\s*", "", t)
    t = re.sub(r"[ \t]+", " ", t)
    t = re.sub(r"\n{3,}", "\n\n", t)
    return t.strip()


def extract_pdf_text(path: Path) -> str:
    reader = PdfReader(str(path))
    chunks = []
    for page in reader.pages:
        try:
            chunks.append(page.extract_text() or "")
        except Exception:
            continue
    return clean_pdf_text("\n".join(chunks))


def split_paragraphs(text: str) -> list[str]:
    paras = [p.strip() for p in re.split(r"\n\s*\n", text) if len(p.strip()) > 40]
    if len(paras) < 3:
        paras = [p.strip() for p in text.split(". ") if len(p.strip()) > 60]
    return paras


def pick_paragraphs(paras: list[str], keywords: tuple[str, ...], limit: int = 6) -> list[str]:
    out = []
    for p in paras:
        low = p.lower()
        if any(k in low for k in keywords):
            out.append(p[:1200])
        if len(out) >= limit:
            break
    return out


def numeric_facts(text: str, limit: int = 35) -> list[str]:
    seen = set()
    facts = []
    for line in text.split("\n"):
        line = line.strip()
        if len(line) < 12 or len(line) > 400:
            continue
        if not NUM_LINE_RE.search(line):
            continue
        if not UNIT_RE.search(line) and line.count("%") == 0:
            if not re.search(r"\d+[\s,\.]\d+", line):
                continue
        key = line[:80]
        if key in seen:
            continue
        seen.add(key)
        facts.append(f"- {line[:350]}")
        if len(facts) >= limit:
            break
    return facts


def norm_text(s: str) -> str:
    return s.replace("\r\n", "\n").replace("\r", "\n").strip()


def build_article_body(item: dict, pdf_text: str) -> str:
    item = {k: norm_text(v) if isinstance(v, str) else v for k, v in item.items()}
    pdf_text = norm_text(pdf_text)
    paras = split_paragraphs(pdf_text) if pdf_text else []
    goal = pick_paragraphs(paras, ("цель", "задач", "актуальност"), 3)
    methods = pick_paragraphs(
        paras,
        ("материал", "метод", "опыт", "сорт", "подвой", "посадк", "повторност"),
        5,
    )
    results = pick_paragraphs(
        paras,
        ("результат", "установлен", "показател", "урожай", "выявлен"),
        6,
    )
    practice = pick_paragraphs(
        paras,
        ("вывод", "рекоменд", "практик", "целесообраз", "следует"),
        4,
    )
    numbers = numeric_facts(pdf_text or item.get("abstract", ""))

    title = norm_text(item["title"]).split(" - Авторы:")[0].strip()
    lines = [
        "Метаданные источника:",
        f"- Заголовок: {title}",
        f"- URL: {item['pdf_url']}",
    ]
    if item.get("doi"):
        lines.append(f"- DOI: https://doi.org/{item['doi']}")
    if item.get("authors"):
        lines.append(f"- Авторы: {item['authors']}")
    lines.append(f"- Культура (RAG): {item['crop_id']}")
    lines.append("")

    abstract = (item.get("abstract") or "").strip()
    abstract = re.sub(r"^(?:и ассистента:\s*)+", "", abstract, flags=re.I)
    if abstract:
        lines.append("Кратко для садовода и ассистента:")
        if len(abstract) > 900:
            lines.append(abstract[:900] + "…")
            lines.append("")
            lines.append("Реферат (полный):")
            lines.append(abstract)
        else:
            lines.append(abstract)
        lines.append("")

    if goal:
        lines.append("Цель и задачи:")
        lines.extend(goal)
        lines.append("")

    if methods:
        lines.append("Объекты, методы и условия:")
        lines.extend(methods)
        lines.append("")

    if results:
        lines.append("Основные результаты:")
        lines.extend(results)
        lines.append("")

    if numbers:
        lines.append("Цифры из текста и таблиц (по PDF):")
        lines.extend(numbers)
        lines.append("")

    if practice:
        lines.append("Практические выводы:")
        lines.extend(practice)
        lines.append("")

    if pdf_text and len(pdf_text) > 500:
        extra = pick_paragraphs(paras, ("яблон", "груш", "слив", "подвой", "сад"), 4)
        if extra:
            lines.append("Дополнительно из полного текста:")
            lines.extend(extra)
            lines.append("")

    lines.append(
        "Дисклеймер: конспект для RAG по открытой публикации; "
        "препараты, дозы и нормы — только по официальной инструкции и законодательству РФ. "
        "Перенос опыта на другой участок требует местной адаптации."
    )
    return "\n".join(lines) + "\n"


def download_pdf(url: str, dest: Path, session: requests.Session) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    if dest.exists() and dest.stat().st_size > 2000:
        return
    r = session.get(url, timeout=120)
    r.raise_for_status()
    dest.write_bytes(r.content)


def load_titles() -> dict:
    if TITLES_PATH.exists():
        return json.loads(TITLES_PATH.read_text(encoding="utf-8"))
    return {"apple": {}, "pear": {}, "plum": {}}


def save_titles(titles: dict) -> None:
    TITLES_PATH.write_text(
        json.dumps(titles, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def ingest_batch(candidates: list[dict], limit: int, dry_run: bool) -> list[dict]:
    session = requests.Session()
    session.headers["User-Agent"] = "doctor_gardens_ai-kb-bot/1.0 (research; local)"
    titles = load_titles()
    have_pdf = existing_pdf_urls()
    written = []

    batch = candidates if limit <= 0 else candidates[:limit]
    for item in batch:
        if item["pdf_url"] in have_pdf:
            continue
        crop = item["crop_id"]
        num = next_article_num(crop)
        slug = slug_from_pdf(item["pdf_url"])
        fname = f"article{num}_{slug}.txt"
        out_path = DATA / crop / fname

        pdf_path = pdf_cache_path(item["pdf_url"])

        try:
            if not dry_run:
                download_pdf(item["pdf_url"], pdf_path, session)
                pdf_text = extract_pdf_text(pdf_path)
            else:
                pdf_text = ""
        except Exception as e:
            print(f"skip {item['pdf_url']}: {e}")
            continue

        body = build_article_body(item, pdf_text)
        if dry_run:
            print(f"[dry] {crop}/{fname} ({len(body)} chars)")
            written.append({"crop": crop, "file": fname, "pdf_url": item["pdf_url"]})
            continue

        out_path.write_text(body, encoding="utf-8")
        have_pdf.add(item["pdf_url"])
        short_title = item["title"][:120]
        titles.setdefault(crop, {})[fname] = short_title
        written.append({"crop": crop, "file": fname, "pdf_url": item["pdf_url"]})
        print(f"ok {crop}/{fname}")
        time.sleep(0.5)

    if written and not dry_run:
        save_titles(titles)
    return written


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--catalog-pages", type=int, default=0, help="пересобрать каталог (0=только файл)")
    p.add_argument("--pages", type=int, default=40, help="страниц каталога при сборе")
    p.add_argument(
        "--limit",
        type=int,
        default=0,
        help="сколько статей за запуск (0 = весь каталог; reindex не вызывается)",
    )
    p.add_argument(
        "--all",
        action="store_true",
        help="обработать весь каталог (то же что --limit 0)",
    )
    p.add_argument("--dry-run", action="store_true")
    p.add_argument(
        "--crop",
        choices=["apple", "pear", "plum", "all"],
        default="all",
        help="фильтр культуры в пачке",
    )
    args = p.parse_args()

    if args.catalog_pages or not CATALOG_PATH.exists():
        pages = args.catalog_pages or args.pages
        items = discover(max_pages=pages, start_page=1)
        CATALOG_PATH.parent.mkdir(parents=True, exist_ok=True)
        CATALOG_PATH.write_text(
            json.dumps(items, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        print(f"catalog refreshed: {len(items)}")
    else:
        items = json.loads(CATALOG_PATH.read_text(encoding="utf-8"))

    if args.crop != "all":
        items = [i for i in items if i.get("crop_id") == args.crop]

    # приоритет: свежие PDF (путь 26/ > 25/ > …)
    def sort_key(it: dict) -> tuple:
        m = re.search(r"/pdf/(\d+)/(\d+)/", it["pdf_url"])
        if m:
            return (-int(m.group(1)), -int(m.group(2)))
        return (0, 0)

    items.sort(key=sort_key)
    limit = 0 if args.all else args.limit
    written = ingest_batch(items, limit, args.dry_run)
    print(f"batch done: {len(written)} articles")


if __name__ == "__main__":
    main()
