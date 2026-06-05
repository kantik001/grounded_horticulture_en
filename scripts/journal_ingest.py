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


def join_wrapped_lines(raw: str) -> str:
    """Склеить переносы PDF в связные абзацы (без обрыва «товарн»)."""
    lines = raw.replace("\r", "\n").split("\n")
    blocks: list[str] = []
    buf = ""
    for line in lines:
        line = re.sub(r"[ \t]+", " ", line.strip())
        if not line:
            if buf:
                blocks.append(buf)
                buf = ""
            continue
        if re.search(
            r"journal(?:kubansad|\.kubansad)\.ru/pdf/|Fruit growing and viticulture",
            line,
            re.I,
        ):
            if buf:
                blocks.append(buf)
                buf = ""
            continue
        if not buf:
            buf = line
            continue
        if buf.endswith("-"):
            buf = buf[:-1] + line
        elif not re.search(r"[.!?;:»\"\)]$", buf) and len(line) < 160:
            buf = f"{buf} {line}"
        else:
            blocks.append(buf)
            buf = line
    if buf:
        blocks.append(buf)
    return "\n\n".join(blocks)


def clean_pdf_text(raw: str) -> str:
    t = raw.replace("\r", "\n")
    t = re.sub(r"-\s*\n\s*", "", t)
    t = join_wrapped_lines(t)
    t = re.sub(r"[ \t]+", " ", t)
    t = re.sub(r"\n{3,}", "\n\n", t)
    return t.strip()


def clip_text(text: str, max_len: int = 2800) -> str:
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) <= max_len:
        return text
    chunk = text[:max_len]
    for pat in (r"[^.!?]+[.!?]\s*", r"[^,;]+[,;]\s*"):
        matches = list(re.finditer(pat, chunk))
        if matches:
            end = matches[-1].end()
            if end > max_len * 0.55:
                return text[:end].strip()
    sp = chunk.rfind(" ")
    if sp > max_len * 0.6:
        return text[:sp].strip() + "…"
    return chunk.strip() + "…"


def extract_abstract_from_pdf(text: str) -> str:
    m = re.search(
        r"Аннотация\.\s*(.+?)(?=Ключевые слова|Key words|Введение|1\.\s*Введение|\nВведение)",
        text,
        re.DOTALL | re.I,
    )
    if not m:
        return ""
    return clip_text(re.sub(r"\s+", " ", m.group(1)), 3500)


def extract_title_from_pdf(text: str) -> str:
    """Заголовок из блока «Для цитирования» без тяжёлого regex."""
    idx = text.lower().find("для цитирования:")
    if idx == -1:
        return ""
    snippet = text[idx + len("для цитирования:") : idx + 1500]
    end_m = re.search(r"\.\s*Плодоводство", snippet, re.I)
    if not end_m:
        return ""
    cite = re.sub(r"\s+", " ", snippet[: end_m.start()]).strip().rstrip(".")
    segs = [s.strip() for s in cite.split(",") if s.strip()]
    if len(segs) < 2:
        return ""
    title = segs[-1]
    title = re.sub(r"^[А-ЯЁA-Z]\.\s*[А-ЯЁ]\.,\s*", "", title)
    if 12 < len(title) < 200:
        return title
    return ""


def extract_pdf_text(path: Path, max_pages: int = 18) -> str:
    reader = PdfReader(str(path))
    chunks = []
    for i, page in enumerate(reader.pages):
        if i >= max_pages:
            break
        try:
            chunks.append(page.extract_text() or "")
        except Exception:
            continue
    return clean_pdf_text("\n".join(chunks))


def split_paragraphs(text: str) -> list[str]:
    paras = [re.sub(r"\s+", " ", p.strip()) for p in re.split(r"\n\s*\n", text) if len(p.strip()) > 50]
    if len(paras) < 3:
        paras = [
            re.sub(r"\s+", " ", s.strip())
            for s in re.split(r"(?<=[.!?])\s+", text)
            if len(s.strip()) > 80
        ]
    return paras


def pick_paragraphs(
    paras: list[str],
    keywords: tuple[str, ...],
    limit: int = 6,
    max_len: int = 2800,
) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for p in paras:
        low = p.lower()
        if low.startswith("аннотация.") or low.startswith("ключевые слова"):
            continue
        if not any(k in low for k in keywords):
            continue
        clipped = clip_text(p, max_len)
        key = clipped[:120]
        if key in seen:
            continue
        seen.add(key)
        out.append(clipped)
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
        facts.append(f"- {clip_text(line, 400)}")
        if len(facts) >= limit:
            break
    return facts


def norm_text(s: str) -> str:
    return s.replace("\r\n", "\n").replace("\r", "\n").strip()


def _needs_pdf_title(title: str) -> bool:
    t = title.strip()
    if not t or t.startswith("article"):
        return True
    if re.match(r"^(?:УДК|UDC)\b", t, re.I):
        return True
    if re.match(r"^Плодоводство", t, re.I):
        return True
    return len(t) < 15


def build_article_body(item: dict, pdf_text: str) -> str:
    item = {k: norm_text(v) if isinstance(v, str) else v for k, v in item.items()}
    pdf_text = clean_pdf_text(pdf_text) if pdf_text else ""
    paras = split_paragraphs(pdf_text) if pdf_text else []

    abstract = (item.get("abstract") or "").strip()
    if not abstract or len(abstract) < 120:
        abstract = extract_abstract_from_pdf(pdf_text)
    abstract = re.sub(r"^(?:и ассистента:\s*)+", "", abstract, flags=re.I)

    goal = pick_paragraphs(paras, ("цель и задач", "целью работ", "актуальност"), 2)
    methods = pick_paragraphs(
        paras,
        ("материал", "метод", "объект", "опыт", "посадк", "повторност"),
        4,
    )
    results = pick_paragraphs(
        paras,
        ("результат", "установлен", "показал", "урожай", "выявлен", "обсужден"),
        5,
    )
    practice = pick_paragraphs(
        paras,
        ("вывод", "рекоменд", "практик", "целесообраз", "следует"),
        3,
    )
    numbers = numeric_facts((pdf_text or abstract)[:80000])

    title = norm_text(item["title"]).split(" - Авторы:")[0].strip()
    if _needs_pdf_title(title):
        pdf_title = extract_title_from_pdf(pdf_text)
        if pdf_title:
            title = pdf_title
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

    if abstract:
        lines.append("Кратко для садовода и ассистента:")
        lines.append(clip_text(abstract, 2200))
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
