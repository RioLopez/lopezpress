#!/usr/bin/env python3
"""doc_check.py -- keep MASTER.md honest.

MASTER.md is the single entry point for this repository: it claims to index
every document and to be the thing a fresh session reads first. The failure
mode is silent: someone (human or AI) adds a document, forgets the index, and
the next session never finds it -- the master doc becomes a lie that nobody
notices.

This script makes that failure loud. It checks four invariants and exits
non-zero on any violation, so it can be wired into the verification gate for
documentation-touching changes.

    1. Doc-count agreement  -- the totals MASTER states everywhere match reality.
    2. Top-level coverage   -- every root / deploy / docs/*.md file is named in MASTER.
    3. Bulk directories     -- specs/ decisions/ prompts/ reference/ archive/ are indexed.
    4. Class banners        -- every .md carries a "> 类别 A|B" banner.

Standard library only: no pip, no network, runs anywhere Python 3 lives.

Usage:
    python scripts/doc_check.py            # human-readable, exit 1 on failure
    python scripts/doc_check.py --quiet    # only failures + final verdict
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MASTER = ROOT / "MASTER.md"

# Directories that hold generated code or local-only state, never project docs.
SKIP_DIRS = {".git", "node_modules", "dist", ".workbuddy", "__pycache__", ".vite"}

# Directories whose contents are indexed as a group (a range entry in MASTER)
# rather than file by file.
BULK_DIRS = ["docs/specs/", "docs/decisions/", "docs/prompts/", "docs/reference/", "docs/archive/"]

# Where every single file must be named explicitly.
EXPLICIT_GLOBS = ["*.md", "deploy/**/*.md", "docs/*.md"]

failures: list[str] = []
quiet = "--quiet" in sys.argv


def say(line: str = "") -> None:
    if not quiet:
        print(line)


def fail(msg: str) -> None:
    failures.append(msg)
    print(f"[FAIL] {msg}")


def collect_docs() -> list[Path]:
    out: list[Path] = []
    for path in ROOT.rglob("*.md"):
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        out.append(path)
    return sorted(out)


def rel(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def main() -> int:
    if not MASTER.exists():
        print("[FAIL] MASTER.md is missing; it is the entry point for this repo")
        return 1

    master = MASTER.read_text(encoding="utf-8")
    docs = collect_docs()
    total = len(docs)

    print("doc_check -- MASTER.md vs reality")
    print("-" * 64)
    say(f"[info] 扫描到 {total} 份 .md（已排除 {', '.join(sorted(SKIP_DIRS))}）")

    # ---------------------------------------------------------------- 1. count
    # MASTER states the total in three different sentences on purpose (section 3
    # header, the arithmetic line, and the section 7 baseline). If they ever
    # disagree with each other, at least one of them is stale.
    claims = set()
    for pattern in (r"共\s*(\d+)\s*份", r"仓库内\s*(\d+)\s*份", r"=\s*\*\*(\d+)\*\*"):
        claims.update(int(n) for n in re.findall(pattern, master))
    if not claims:
        fail("MASTER 里没找到任何文档数量声明（共 N 份 / 仓库内 N 份 / = **N**）")
    elif claims != {total}:
        fail(f"文档数量不一致：实际 {total}，MASTER 声明 {sorted(claims)}")
    else:
        say(f"[ok] 文档总数 {total} 与 MASTER 全部声明一致")

    # ------------------------------------------------------- 2. top-level named
    explicit: list[Path] = []
    for pattern in EXPLICIT_GLOBS:
        for path in ROOT.glob(pattern):
            if path.is_file() and path not in explicit:
                explicit.append(path)
    for bulk in BULK_DIRS:
        explicit = [p for p in explicit if not rel(p).startswith(bulk)]

    missing = [rel(p) for p in explicit if rel(p) not in master]
    if missing:
        for name in missing:
            fail(f"文档未在 MASTER §3 中出现：{name}")
    else:
        say(f"[ok] 顶层文档全部被点名（{len(explicit)}/{len(explicit)}）")

    # -------------------------------------------------------------- 3. bulk dirs
    absent = [d for d in BULK_DIRS if d not in master]
    if absent:
        for d in absent:
            fail(f"批量目录未在 MASTER 中被索引：{d}")
    else:
        say(f"[ok] 批量目录均已索引（{len(BULK_DIRS)}/{len(BULK_DIRS)}）")

    # ----------------------------------------------------------- 4. A/B banners
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

    # --------------------------------------------------------------- verdict
    print("-" * 64)
    if failures:
        print(f"RESULT: FAIL ({len(failures)} 项)")
        print("修法：把缺的文档补进 MASTER §3（并重算份数），或给文件补横幅。")
        return 1
    print("RESULT: PASS -- 母文档与实际一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())
