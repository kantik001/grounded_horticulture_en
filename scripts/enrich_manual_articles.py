#!/usr/bin/env python3
"""Углубление вручную подготовленных статей: база из git HEAD + данные из PDF журнала."""
from __future__ import annotations

import json
import re
import subprocess
import sys
import time
from pathlib import Path

import requests
from journal_ingest import pdf_cache_path
from pypdf import PdfReader

_SCRIPTS = Path(__file__).resolve().parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

ROOT = _SCRIPTS.parent
DATA = ROOT / "data"
PDF_DIR = ROOT / "_tmp" / "journal_pdfs"
TITLES_PATH = ROOT / "config" / "article_titles.json"
CATALOG = ROOT / "_tmp" / "journal_catalog.json"

# article1 — вручную, URL не был в файле
EXTRA_MANUAL = [
    "data/apple/article1_orfey_margo_freestanding.txt",
]

UNIT_RE = re.compile(
    r"(т/га|кг/га|г/л|л/га|мм|см|м\b|%|°C|балл|ц/га|мг/кг|мг/100|дер\./га|л/дерев)",
    re.I,
)


def git_head_text(path: str) -> str:
    r = subprocess.run(
        ["git", "show", f"HEAD:{path}"],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="ignore",
    )
    return r.stdout if r.returncode == 0 else ""


def discover_manual_paths() -> list[str]:
    found = list(EXTRA_MANUAL)
    for crop in ("apple", "pear", "plum"):
        r = subprocess.run(
            ["git", "ls-tree", "-r", "HEAD", "--name-only", f"data/{crop}"],
            capture_output=True,
            text=True,
        )
        for p in r.stdout.strip().splitlines():
            if not p.endswith(".txt"):
                continue
            t = git_head_text(p)
            if "journalkubansad.ru/pdf/" not in t:
                continue
            if "Кратко для садовода" in t and "Цель и задачи:" in t:
                continue
            if "Кратко:" in t or "Практика:" in t or "Новое для" in t or "Кратко\n" in t:
                found.append(p)
    return sorted(set(found))


def pdf_url_from_text(text: str) -> str:
    m = re.search(r"https?://journalkubansad\.ru/pdf/[^\s\)\]]+", text)
    return m.group(0).rstrip(").,") if m else ""


def topic_hints(head: str) -> tuple[list[str], list[str]]:
    """Ключевые слова темы и «чужие» культуры для фильтрации PDF."""
    want: list[str] = []
    avoid: list[str] = []
    blob = head.lower()
    for w in re.findall(r"[а-яёa-z]{5,}", blob):
        if w in (
            "журнал",
            "авторы",
            "культуры",
            "приоритет",
            "важно",
            "пробелов",
            "регион",
            "ассистента",
            "садовода",
        ):
            continue
        if w not in want and len(want) < 14:
            want.append(w)
    crops = re.search(r"культуры:\s*([^\n]+)", head, re.I)
    if crops:
        c = crops.group(1).lower()
        if "яблон" in c or "виноград" in c:
            if "черешн" not in c and "слив" not in c:
                avoid.extend(["черешн", "вишн", "абрикос", "персик"])
        if "груш" in c and "яблон" not in c:
            avoid.extend(["яблон", "марссони", "плодожорк"])
        if "слив" in c and "яблон" not in c:
            avoid.extend(["яблон", "марссони", "орфей"])
    if "marssonina" in blob or "марссони" in blob or "фитосанит" in blob:
        want.extend(["marssonina", "марссони", "фитосанит", "плодожорк", "виноград"])
        avoid.extend(["черешн"])
    return want, avoid


def line_relevant(line: str, want: list[str], avoid: list[str]) -> bool:
    low = line.lower()
    if avoid and any(a in low for a in avoid):
        return False
    if want and not any(w in low for w in want):
        return False
    return True


def strip_pdf_supplements(text: str) -> str:
    markers = (
        "\n\nЦифры из таблиц и текста PDF:",
        "\n\nДополнение — цифры из PDF:",
        "\n\nАннотация из журнала (для сверки):",
        "\n\nОсновные результаты (по PDF):",
        "\n\nПрактика — дополнение из PDF:",
        "\n\nПрактические выводы:\n",
        "\n\nДисклеймер: дополнено по открытой публикации",
    )
    cut = len(text)
    for m in markers:
        i = text.find(m)
        if i != -1:
            cut = min(cut, i)
    return text[:cut].rstrip()


def extract_pdf(path: Path) -> str:
    reader = PdfReader(str(path))
    parts = []
    for page in reader.pages:
        try:
            parts.append(page.extract_text() or "")
        except Exception:
            pass
    t = "\n".join(parts).replace("\r", "\n")
    t = re.sub(r"-\s*\n\s*", "", t)
    t = re.sub(r"[ \t]+", " ", t)
    return t


def download_pdf(url: str, dest: Path, session: requests.Session) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    if dest.exists() and dest.stat().st_size > 2000:
        return
    r = session.get(url, timeout=120)
    r.raise_for_status()
    dest.write_bytes(r.content)


def numeric_bullets(text: str, limit: int = 28) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for line in text.split("\n"):
        line = line.strip()
        if len(line) < 18 or len(line) > 320:
            continue
        if not re.search(r"\d", line):
            continue
        if not UNIT_RE.search(line) and "%" not in line:
            continue
        if re.search(r"^(Рис\.|Fig\.|doi\.org|journalkubansad)", line, re.I):
            continue
        if re.search(r"20\d{2};\d+\(\d+\):|Fruit growing and viticulture", line):
            continue
        key = line[:70]
        if key in seen:
            continue
        seen.add(key)
        out.append(f"- {line}")
        if len(out) >= limit:
            break
    return out


def extract_abstract_pdf(text: str, want: list[str] | None = None, avoid: list[str] | None = None) -> str:
    want = want or []
    avoid = avoid or []
    best = ""
    best_score = -1
    for m in re.finditer(
        r"Аннотация\.\s*(.+?)(?=Ключевые слова|Введение|For citation|1\.\s*Введение|\nВведение)",
        text,
        re.DOTALL | re.I,
    ):
        chunk = re.sub(r"\s+", " ", m.group(1)).strip()[:2000]
        if avoid and any(a in chunk.lower() for a in avoid):
            continue
        score = sum(1 for w in want if w in chunk.lower())
        if score > best_score:
            best_score = score
            best = chunk
    return best


def practice_from_pdf(text: str) -> list[str]:
    bullets: list[str] = []
    for hdr in ("Выводы", "Заключение", "Практические рекомендации"):
        if hdr not in text:
            continue
        chunk = text.split(hdr, 1)[1][:2500]
        for sent in re.split(r"(?<=[.!?])\s+", chunk):
            s = sent.strip()
            if 50 < len(s) < 400 and re.search(r"(рекоменд|следует|целесообраз|необходим|урожай|сорт|подвой|яблон|груш|слив)", s, re.I):
                bullets.append(f"- {s}")
        if bullets:
            break
    return bullets[:8]


def results_paragraphs(text: str, limit: int = 4) -> list[str]:
    paras: list[str] = []
    for pat in (
        r"Установлено,[^.]{40,500}\.",
        r"Полученные результаты[^.]{40,500}\.",
        r"максимальн[^.]{40,400}\.",
        r"урожайност[^.]{40,400}\.",
    ):
        for m in re.finditer(pat, text, re.I):
            p = re.sub(r"\s+", " ", m.group(0)).strip()
            if p not in paras:
                paras.append(p)
            if len(paras) >= limit:
                return paras
    return paras


def enrich_body(base: str, pdf_text: str) -> str:
    """HEAD-версия + блоки из PDF, без замены ручной структуры."""
    want, avoid = topic_hints(base)
    out = strip_pdf_supplements(base.strip())
    # убрать следы auto-ingest, если файл уже перезаписан restore
    if "Цель и задачи:" in out and "Кратко:" not in out and "Кратко для садовода" in out:
        head = git_head_text("")  # noop
    if "Реферат (полный):" in out or (
        "Цель и задачи:" in out and len(out) > 4000
    ):
        # слишком похоже на ingest — не трогаем здесь, вызывающий код подставит HEAD
        pass

    numbers = [
        n
        for n in numeric_bullets(pdf_text, limit=35)
        if line_relevant(n[2:] if n.startswith("- ") else n, [], avoid)
    ]
    results = [
        p
        for p in results_paragraphs(pdf_text)
        if line_relevant(p, want, avoid) or (not avoid and len(p) > 60)
    ]
    practice = [
        b
        for b in practice_from_pdf(pdf_text)
        if line_relevant(b[2:] if b.startswith("- ") else b, [], avoid)
    ]

    # ручные блоки «Кратко», «Новое для региона», «Практика» не перезаписываем — только дополняем

    existing_nums = set(re.findall(r"^- .+", out, re.M))
    new_nums = [n for n in numbers if n not in existing_nums]
    if new_nums:
        if "Цифры из таблиц" in out or "Цифры из текста" in out:
            out += "\n\nДополнение — цифры из PDF:\n" + "\n".join(new_nums[:20])
        else:
            out += "\n\nЦифры из таблиц и текста PDF:\n" + "\n".join(new_nums[:25])

    pdf_abstract = extract_abstract_pdf(pdf_text, want, avoid)
    if pdf_abstract and "Аннотация из журнала (для сверки)" not in out:
        out += "\n\nАннотация из журнала (для сверки):\n" + pdf_abstract[:1500]

    if results and "Основные результаты" not in out:
        out += "\n\nОсновные результаты (по PDF):\n"
        for p in results:
            out += p + "\n"

    if practice and "дополнение из PDF" not in out.lower():
        if "Практика:" in out or "Практические выводы" in out:
            out += "\n\nПрактика — дополнение из PDF:\n" + "\n".join(practice)
        else:
            out += "\n\nПрактические выводы:\n" + "\n".join(practice)

    if "Дисклеймер" not in out:
        out += (
            "\n\nДисклеймер: дополнено по открытой публикации journalkubansad.ru; "
            "препараты и дозы — по официальной инструкции и законодательству РФ."
        )
    return out.strip() + "\n"


def resolve_path(crop: str, head_path: str) -> Path | None:
    """Файл мог быть переименован — ищем по URL из HEAD."""
    head = git_head_text(head_path)
    url = pdf_url_from_text(head)
    crop_dir = DATA / crop
    if url:
        for p in crop_dir.glob("article*.txt"):
            if url in p.read_text(encoding="utf-8", errors="ignore"):
                return p
    p = ROOT / head_path
    return p if p.exists() else None


def title_from_head(head: str, path: Path) -> str:
    m = re.search(r"-\s*Журнал:.*", head)
    m2 = re.search(r"-\s*Заголовок:\s*(.+)", head)
    if m2:
        return m2.group(1).strip()[:120]
    # из имени файла
    stem = path.stem.split("_", 1)
    return path.name


def main() -> None:
    paths = discover_manual_paths()
    session = requests.Session()
    session.headers["User-Agent"] = "doctor_gardens_ai-kb-bot/1.0 (research; local)"
    titles = json.loads(TITLES_PATH.read_text(encoding="utf-8")) if TITLES_PATH.exists() else {}

    # article1: URL по DOI номеру 98-2-136-146 → pdf/26/02/10.pdf
    article1_url = "http://journalkubansad.ru/pdf/26/02/10.pdf"

    done = 0
    for head_path in paths:
        crop = head_path.split("/")[1]
        head = git_head_text(head_path)
        if not head.strip():
            print(f"skip no HEAD {head_path}")
            continue
        target = resolve_path(crop, head_path)
        if not target:
            print(f"skip missing {head_path}")
            continue

        url = pdf_url_from_text(head)
        if head_path.endswith("article1_orfey_margo_freestanding.txt"):
            url = article1_url

        if not url:
            print(f"skip no url {head_path}")
            continue

        pdf_path = pdf_cache_path(url)
        try:
            download_pdf(url, pdf_path, session)
            pdf_text = extract_pdf(pdf_path)
        except Exception as e:
            print(f"fail pdf {target.name}: {e}")
            continue

        body = enrich_body(head, pdf_text)
        target.write_text(body, encoding="utf-8")

        m = re.search(r"Кратко(?: для садовода и ассистента)?:?\s*\n(.+)", head, re.DOTALL)
        if m:
            nice = re.sub(r"\s+", " ", m.group(1))[:115].strip()
            if len(nice) > 112:
                nice = nice[:112] + "…"
        else:
            nice = target.stem
        if crop == "apple" and not nice.lower().startswith("яблон"):
            if re.search(r"яблон|подво|сад|марссони|плодожор", nice, re.I):
                nice = "Яблоня: " + nice[0].lower() + nice[1:]
        titles.setdefault(crop, {})[target.name] = nice
        print(f"ok {target.name} ({len(body)} chars)")
        done += 1
        time.sleep(0.25)

    TITLES_PATH.write_text(
        json.dumps(titles, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"enriched {done} manual articles")


if __name__ == "__main__":
    main()
