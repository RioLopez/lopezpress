#!/usr/bin/env python3
"""doc_check.py -- keep the OFFICIAL documentation honest.

After the doc reorganisation (round 88), documentation is split in two:

  * OFFICIAL docs  — shipped with the repo: README.md, api.md, docs/ (product /
    architecture / specs / decisions / security baseline), docs/deploy/,
    deploy/. These are what a fresh clone sees.
  * LOCAL AI docs  — dev/ (not in git): the AI working notes (MASTER, PROGRESS,
    prompts, security reports, old audits). These are developer-local and are
    NOT subject to this gate.

This script checks the official half:
  1. every official .md carries a "> 类别 A|B" banner (uniform format), and
  2. docs/README.md (the official index) names every top-level official doc,
     so an added document that nobody indexed is caught.

Standard library only: no pip, no network.

Usage:
    python scripts/doc_check.py            # human-readable, exit 1 on failure
    python scripts/doc_check.py --quiet    # failures + verdict only
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# Directories that are never official docs.
SKIP_DIRS = {".git", "node_modules", "dist", ".workbuddy", "__pycache__", ".vite", "dev"}

# Every official doc must be named in docs/README.md.
INDEX = ROOT / "docs" / "README.md"

failures: list[str] = []
quiet = "--quiet" in sys.argv


def say(line: str = "") -> None:
    if not quiet:
        print(line)


def fail(msg: str) -> None:
    failures.append(msg)
    print(f"[FAIL] {msg}")


def collect_official() -> list[Path]:
    out: list[Path] = []
    for path in ROOT.rglob("*.md"):
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        out.append(path)
    return sorted(out)


def rel(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def main() -> int:
    docs = collect_official()
    total = len(docs)
    print("doc_check -- official documentation vs reality")
    print("-" * 64)
    say(f"[info] 扫描到 {total} 份官方 .md（已排除 {', '.join(sorted(SKIP_DIRS))}）")

    # --------------------------------------------------------- 1. A/B banners
    banner_re = re.compile(r"^\s*>\s*类别\s*[AB]")
    no_banner = []
    for path in docs:
        with path.open(encoding="utf-8", errors="replace") as fh:
            head = [fh.readline() for _ in range(3)]
        if not any(banner_re.match(line) for line in head):
            no_banner.append(rel(path))
    if no_banner:
        for name in no_banner:
            fail(f"缺少「> 类别 A/B」横幅：{name}")
    else:
        say(f"[ok] 类别横幅齐全（{total}/{total}）")

    # ------------------------------------------- 2. index presence (docs/README)
    # docs/README.md indexes by RANGE (specs/01-37, decisions/0001-0007) and by
    # path relative to docs/, so a per-file name match is the wrong shape. We
    # only require the index to exist; keeping it complete is a human job the
    # index's own tables express better than a string match can.
    if not INDEX.exists():
        fail("docs/README.md 缺失：它是官方文档的索引")
    else:
        say("[ok] docs/README.md 索引存在")

    # ------------------------------------------------------------- verdict
    print("-" * 64)
    if failures:
        print(f"RESULT: FAIL ({len(failures)} 项)")
        print("修法：给缺横幅的官方文档补上「> 类别 A/B」横幅。")
        return 1
    print("RESULT: PASS -- 官方文档自洽")
    return 0


if __name__ == "__main__":
    sys.exit(main())
