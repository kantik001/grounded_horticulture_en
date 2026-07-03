#!/usr/bin/env python3
"""Targeted cleanup: only clearly damaged files and junk in Figures blocks."""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data"

CORRUPT_MARK = re.compile(
    r"Brieffor|activitiesfor|Varietyinfluence|fruitsetas|Figuresfrom|havinggen|"
    r"Krasnodarregion|varietyapple|areainall|productivityand",
    re.I,
)

HEADER_IN_LINE = re.compile(
    r"20\d{2};\d+\(\d+\):\d|Fruit growing and viticulture|journalkubansad\.ru/pdf/",
    re.I,
)

MID_BULLET = re.compile(
    r"^- (?:with|and|to|in|on|at|by|of|or|as|an|a|but|for|from|not|no)\s",
    re.I,
)


def clean_supplement_bullets(text: str) -> tuple[str, int]:
    """Remove broken bullets only in enrichment supplement blocks."""
    removed = 0
    lines = text.splitlines()
    in_block = False
    for i, line in enumerate(lines):
        if line.startswith(
            (
                "Figures from tables",
                "Supplement — figures",
                "Abstract from journal",
            )
        ):
            in_block = True
            continue
        if in_block and line.startswith(
            (
                "Main findings",
                "Practice — supplement",
                "Disclaimer:",
                "Practice:",
                "Brief",
                "New for",
                "Source metadata",
                "Metadata",
            )
        ):
            in_block = False
        if not in_block or not line.strip().startswith("- "):
            continue
        if HEADER_IN_LINE.search(line) or MID_BULLET.match(line.strip()):
            lines[i] = ""
            removed += 1
    return "\n".join(lines), removed


def main() -> int:
    """Clean corrupted supplement bullets in crop articles and report counts."""
    changed = fixes = 0
    for crop in ("apple", "pear", "plum"):
        for p in sorted((DATA / crop).glob("article*.txt")):
            raw = p.read_text(encoding="utf-8", errors="ignore")
            if not CORRUPT_MARK.search(raw) and "Figures from tables" not in raw:
                continue
            t, r = clean_supplement_bullets(raw)
            if t != raw:
                if not t.endswith("\n"):
                    t += "\n"
                p.write_text(t, encoding="utf-8")
                changed += 1
                fixes += r
    print(f"changed={changed} bullet_cleaned={fixes}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
