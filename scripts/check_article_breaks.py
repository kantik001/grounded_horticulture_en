#!/usr/bin/env python3
"""Check articles for broken line wraps, PDF glue artifacts, and header junk."""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data"

# Truncated line: no .!?;:)" at end, but long and not a header/marker
TRUNC_LINE = re.compile(
    r"^[-•]?\s*.{40,}[a-z0-9]$",
    re.I,
)

# PDF glue: space inside a word (Latin)
SPLIT_WORD = re.compile(r"[a-z]{2,}\s+[a-z]{2,}", re.I)

# Journal header inside a paragraph
HEADER_JUNK = re.compile(
    r"20\d{2};\d+\(\d+\):\d+-\d+|Fruit growing and viticulture|journalkubansad\.ru/pdf/",
    re.I,
)

# Line starts mid-sentence (lowercase after previous period — rough heuristic)
MID_SENT_START = re.compile(
    r"^[a-z][a-z\s]{10,}(?:ing|ion|ment|able|ness|ized|ally|for)\s",
)


def check_file(path: Path) -> list[str]:
    issues: list[str] = []
    text = path.read_text(encoding="utf-8", errors="ignore")
    lines = text.splitlines()

    for i, line in enumerate(lines, 1):
        s = line.strip()
        if not s or len(s) < 25:
            continue
        if s.startswith(
            (
                "Source metadata",
                "Metadata",
                "- Journal",
                "- URL",
                "- DOI",
                "Brief",
                "Goal",
                "Practice:",
            )
        ):
            continue
        if TRUNC_LINE.match(s) and not s.endswith((")", "]", "%", "°C")):
            issues.append(f"L{i}: truncated line: …{s[-55:]}")
        if HEADER_JUNK.search(s) and "URL:" not in s:
            issues.append(f"L{i}: PDF header in text")
        if re.search(r"\b[A-Z][a-z]{1,3}\s+[a-z]{3,}", s):
            pass

    for m in SPLIT_WORD.finditer(text):
        frag = m.group(0)
        if len(frag) < 12 or frag.count(" ") > 1:
            continue
        a, b = frag.split()
        if len(a) >= 4 and len(b) >= 4 and a[-1].isalpha() and b[0].isalpha():
            pos = text.find(frag)
            line_no = text[:pos].count("\n") + 1
            issues.append(f"L{line_no}: split word «{frag}»")
            if len(issues) > 8:
                break

    for i, line in enumerate(lines, 1):
        s = line.strip()
        if s.startswith("- ") and MID_SENT_START.match(s[2:]):
            issues.append(f"L{i}: bullet starts mid-phrase: {s[:70]}…")

    return issues[:12]


def main() -> int:
    bad: list[tuple[str, list[str]]] = []
    for crop in ("apple", "pear", "plum"):
        d = DATA / crop
        if not d.is_dir():
            continue
        for p in sorted(d.glob("article*.txt")):
            issues = check_file(p)
            if issues:
                bad.append((str(p.relative_to(ROOT)), issues))

    print(f"Files checked: {sum(1 for _ in DATA.rglob('article*.txt'))}")
    print(f"With issues: {len(bad)}")
    for rel, issues in bad[:40]:
        print(f"\n{rel}")
        for x in issues:
            print(f"  {x}")
    if len(bad) > 40:
        print(f"\n… and {len(bad) - 40} more files")

    manual_hints = []
    for rel, issues in bad:
        if any("PDF header" in x or "mid-phrase" in x for x in issues):
            manual_hints.append(rel)
    if manual_hints:
        print(f"\nPriority fixes (manual/enriched): {len(manual_hints)}")

    return 1 if len(bad) > 50 else 0


if __name__ == "__main__":
    sys.exit(main())
