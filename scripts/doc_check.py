#!/usr/bin/env python3
"""doc_check.py -- keep the documentation from growing back.

2026-10-27: the repo went from 92 markdown files to 6. The old gate checked
"every doc has an A/B banner" and "docs/README.md exists". With six files, the
real risk is no longer a missing banner -- it is **the pile growing back**.
So this gate is now a whitelist:

  1. every .md in the repo must be listed in ALLOWED (5 official docs),
  2. each of them must carry a "> 类别 A|B" banner,
  3. a missing file is REPORTED but does not fail -- shrinking further is fine.

The 6th file, dev/ai-dev/MASTER.md, is local-only (dev/ is gitignored and in
SKIP_DIRS), so it is not counted here.

Adding a document is therefore a two-step decision: write the file, then add it
to ALLOWED below. That second step is the speed bump that stops the pile.

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

# Directories that are never official docs. dev/ is local AI notes (gitignored).
SKIP_DIRS = {".git", "node_modules", "dist", ".workbuddy", "__pycache__", ".vite", "dev"}

# The complete set of official documents. This list IS the policy: the repo is
# deliberately kept at this size, and every one of them earns its place.
ALLOWED = {
    "README.md",                      # 上手：跑起来、配置、演示账号
    "api.md",                         # 接口参考 + 变更日志
    "deploy/README.md",               # 部署 / 上线 / HTTPS / 排障
    "docs/TODO.md",                   # 唯一待办清单（只列没做完的）
    "docs/SECURITY_BASELINE.md",      # 能不能上线
}

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
    names = {rel(p) for p in docs}
    print("doc_check -- documentation whitelist")
    print("-" * 64)
    say(f"[info] 扫描到 {len(docs)} 份官方 .md（策略上限 {len(ALLOWED)}）")

    # ------------------------------------------------- 1. nothing extra
    extra = sorted(names - ALLOWED)
    if extra:
        for name in extra:
            fail(f"未登记的文档：{name}")
        print()
        print("  2026-10-27 起文档固定为 6 份（5 份官方 + 本地 dev/ai-dev/MASTER.md）。")
        print("  新增文档前先问：能不能写进这 6 份里已有的那一份？")
        print("  确实要独立成文 → 在 scripts/doc_check.py 的 ALLOWED 里加一行，这是有意的决定。")
    else:
        say(f"[ok] 没有多余文档（{len(names)}/{len(ALLOWED)}）")

    # ------------------------------------------------- 2. all still present
    missing = sorted(ALLOWED - names)
    for name in missing:
        say(f"[warn] 缺失：{name}（继续精简没问题，记得同步白名单）")

    # ------------------------------------------------- 3. banners
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
        say(f"[ok] 类别横幅齐全（{len(docs)}/{len(docs)}）")

    # -------------------------------------------------- verdict
    print("-" * 64)
    if failures:
        print(f"RESULT: FAIL ({len(failures)} 项)")
        return 1
    print("RESULT: PASS -- 文档仍在 6 份之内且自洽")
    return 0


if __name__ == "__main__":
    sys.exit(main())
