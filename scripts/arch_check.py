#!/usr/bin/env python3
"""arch_check.py -- the ratchet that keeps architecture drift from growing back.

The 2026-10-23 architecture review (docs/ARCH_REVIEW_2026-10-23.md) found that
this codebase did not get worse by being badly written in any single commit --
it got worse by solving the same problem a different way each round (9 classes
of "one thing, several idioms"). Written rules did not hold, because nothing
mechanically checked them. This script is that mechanism.

Design: a **ratchet**, not a linter. Existing violations are recorded in
scripts/arch_baseline.json. The script fails when a count goes UP, and nudges
(without failing) when a count goes DOWN, so debt can only shrink. That is what
lets the review's 8-stage migration land incrementally without a big-bang
rewrite: every stage lowers the baseline, and nothing can quietly re-grow.

Rules implemented (R6/R7 land in later stages, see the review):

    R1  layering   business packages must not import net/http
    R2  size       file <= 350 lines, function <= 60 lines
    R3  types      entity/DTO structs do not pile up in the service package
    R4  auth       handlers declare no require* checks (route table owns roles)
    R5  audit      handlers write no audit records (use cases own them)
    R7  migration  column migrations live in the registry, not clone funcs
    R8  frontend   exactly one fetch outlet in web/src/core/http.ts
    R9  comments   no duplicated lines inside one doc comment
    R10 audit      action_log rows are written only by services/audit.go

Rule R6 (error construction) lands with P6 -- it needs an AST-level check, not
a grep-level one.

Standard library only: no pip, no network, runs anywhere Python 3 lives.

Usage:
    python scripts/arch_check.py              # compare against baseline
    python scripts/arch_check.py --verbose    # print every violation
    python scripts/arch_check.py --update     # accept current counts as baseline
    python scripts/arch_check.py --baseline   # first run / regenerate baseline

Exit codes: 0 = no rule grew, 1 = a rule grew, 2 = baseline missing.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BASELINE = ROOT / "scripts" / "arch_baseline.json"

# ---------------------------------------------------------------- rule limits
MAX_FILE_LINES = 350
MAX_FUNC_LINES = 60

# R1: packages that must never see the transport. handlers / middleware /
# router / clientip ARE the transport. Includes the target package names so
# files moved during P1/P3 stay clean once they arrive.
BUSINESS_PKGS = {"services", "models", "config", "app", "policy", "store"}

# R1: symbols from net/http that couple a package to the transport. Plain
# http.StatusXxx constants are deliberately NOT in this list: they are untyped
# integers, they cost nothing at runtime, and replacing them with a local copy
# would be a rename with zero architectural gain. What actually breaks layering
# is taking a *http.Request, writing to a ResponseWriter, calling ServeFile, or
# holding an http.Client -- those force a transport to exist before the logic
# can run.
TRANSPORT_SYMBOLS = re.compile(
    r"http\.(?:ResponseWriter|Request\b|Handler\b|HandlerFunc|ServeMux|ServeFile"
    r"|FileServer|NotFound\b|Error\b|Client\b|DefaultClient|NewRequest"
    r"|Method[A-Z]\w*|MaxBytesReader|Hijacker|Flusher)"
)
# R1b (observational, never fails the build): files still importing net/http
# purely for status constants. Tracked so the number is visible if someone
# ever wants to make the domain speak in domain terms instead of HTTP numbers.
STATUS_ONLY_ALLOWED = True

# R8: the only file allowed to call fetch() directly.
FETCH_ALLOWED = {"web/src/core/api.ts", "web/src/core/http.ts"}


def rel(p: Path) -> str:
    return p.relative_to(ROOT).as_posix()


def go_files(sub: str, include_tests: bool = False) -> list[Path]:
    files = sorted((ROOT / sub).rglob("*.go"))
    if not include_tests:
        files = [f for f in files if not f.name.endswith("_test.go")]
    return files


def code_lines(path: Path) -> list[str]:
    return path.read_text(encoding="utf-8", errors="replace").splitlines()


# ---------------------------------------------------------------------- R1
def check_r1() -> tuple[int, list[str]]:
    """Transport symbols inside business packages -- the direction arrow broken.

    Only code lines count: comments that *describe* the old coupling (e.g.
    "used to call http.ServeFile") are documentation, not a dependency.
    """
    hits: list[str] = []
    for f in go_files("internal"):
        pkg = f.relative_to(ROOT / "internal").parts[0]
        if pkg not in BUSINESS_PKGS:
            continue
        for i, line in enumerate(code_lines(f), 1):
            s = line.lstrip()
            if s.startswith("//") or s.startswith("*") or s.startswith("/*"):
                continue
            m = TRANSPORT_SYMBOLS.search(line)
            if m:
                hits.append(f"{rel(f)}:{i} {m.group(0)}")
    return len(hits), hits


def observe_r1_import() -> int:
    """Files that import net/http but only use status constants (informational)."""
    n = 0
    for f in go_files("internal"):
        pkg = f.relative_to(ROOT / "internal").parts[0]
        if pkg not in BUSINESS_PKGS:
            continue
        src = f.read_text(encoding="utf-8", errors="replace")
        if re.search(r'^\s*(?:[\w.]+\s+)?"net/http"', src, re.M):
            n += 1
    return n


# ---------------------------------------------------------------------- R2
def _func_blocks(lines: list[str]) -> list[tuple[int, int, str]]:
    """(start_line, length, signature) for each top-level func, brace-matched.

    Heuristic: braces inside strings are not stripped. Acceptable here because
    the number is a trend indicator, not a compiler.
    """
    out: list[tuple[int, int, str]] = []
    i = 0
    while i < len(lines):
        if lines[i].startswith("func "):
            start = i
            depth = 0
            seen = False
            while i < len(lines):
                depth += lines[i].count("{") - lines[i].count("}")
                if "{" in lines[i]:
                    seen = True
                if seen and depth <= 0:
                    break
                i += 1
            out.append((start + 1, i - start + 1, lines[start][:80]))
        i += 1
    return out


def check_r2() -> tuple[dict[str, int], list[str]]:
    """Size ceilings. Returns two counters plus the offending locations."""
    big_files: list[str] = []
    big_funcs: list[str] = []
    for f in go_files("internal"):
        lines = code_lines(f)
        if len(lines) > MAX_FILE_LINES:
            big_files.append(f"{rel(f)} ({len(lines)} lines)")
        for ln, length, sig in _func_blocks(lines):
            if length > MAX_FUNC_LINES:
                name = sig.replace("func ", "").split("(")[0].strip()
                big_funcs.append(f"{rel(f)}:{ln} {name} ({length} lines)")
    counts = {"R2.files": len(big_files), "R2.funcs": len(big_funcs)}
    return counts, big_files + big_funcs


# ---------------------------------------------------------------------- R3
def check_r3() -> tuple[int, list[str]]:
    """Struct definitions accumulating in the service package.

    The review found 27 DTOs interleaved with methods. They belong in the
    transport DTO file (or app/*/input.go); the entity types in models.
    Service/ServiceError are excluded -- they are the package's own plumbing,
    not data shapes crossing the boundary.
    """
    own = {"Service", "ServiceError"}
    hits: list[str] = []
    for f in go_files("internal/services"):
        for i, line in enumerate(code_lines(f), 1):
            m = re.match(r"^type ([A-Z]\w*)\s+struct\b", line)
            if m and m.group(1) not in own:
                hits.append(f"{rel(f)}:{i} {line.strip()[:60]}")
    return len(hits), hits


# ------------------------------------------------------------------ R4 / R5
def _handler_calls(pattern: str) -> list[str]:
    hits: list[str] = []
    rx = re.compile(pattern)
    for f in go_files("internal/handlers"):
        for i, line in enumerate(code_lines(f), 1):
            s = line.lstrip()
            if s.startswith("//") or s.startswith("func "):
                continue  # comments and the helper's own definition
            if rx.search(line):
                hits.append(f"{rel(f)}:{i} {s[:70]}")
    return hits


def check_r4() -> tuple[int, list[str]]:
    """require* calls inside handlers: roles belong on the route table (P4)."""
    hits = _handler_calls(r"\brequire(?:Admin|Root|Auth)\s*\(")
    return len(hits), hits


def check_r5() -> tuple[int, list[str]]:
    """recordAudit calls inside handlers: use cases own audit (P2)."""
    hits = _handler_calls(r"\brecordAudit\s*\(")
    return len(hits), hits


# ---------------------------------------------------------------------- R7
def check_r7() -> tuple[int, list[str]]:
    """Hand-copied add-column migration functions in the config package.

    Every column migration must be a declarative entry in `var migrations`
    (data), not a bespoke `addXxxColumn` function. The review found 9 clones
    that each re-implemented the gate-check / probe / ALTER / record dance;
    one missed step shipped a month-long 500 (site_url). The generic helper is
    `ensureColumns`; any function named `add<X>Column(s)` is the clone class.
    """
    hits: list[str] = []
    for f in go_files("internal/config"):
        for i, line in enumerate(code_lines(f), 1):
            if re.match(r"^func add[A-Z]\w*Column", line):
                hits.append(f"{rel(f)}:{i} {line.strip()[:60]}")
    return len(hits), hits


# ---------------------------------------------------------------------- R8
def check_r8() -> tuple[int, list[str]]:
    """fetch() calls outside the single network outlet.

    Target state: one file owns the transport primitives, and it calls fetch()
    exactly once. So the allowed file may keep 1 call; every extra call inside
    it and every call outside it counts as debt (the review found 4 in api.ts).
    """
    hits: list[str] = []
    inside: list[str] = []
    for p in sorted((ROOT / "web" / "src").rglob("*")):
        if p.suffix not in {".ts", ".tsx"}:
            continue
        r = rel(p)
        for i, line in enumerate(code_lines(p), 1):
            s = line.lstrip()
            if s.startswith("//") or s.startswith("*"):
                continue
            if re.search(r"(?<![\w.])fetch\s*\(", line):
                where = f"{r}:{i} {s[:70]}"
                (inside if r in FETCH_ALLOWED else hits).append(where)
    if len(inside) > 1:
        hits.extend(inside[1:])
    return len(hits), hits


def collect() -> tuple[dict[str, int], dict[str, list[str]]]:
    counts: dict[str, int] = {}
    details: dict[str, list[str]] = {}

    n, d = check_r1()
    counts["R1"], details["R1"] = n, d
    counts["R1b"] = observe_r1_import()
    c2, d2 = check_r2()
    counts.update(c2)
    details["R2"] = d2
    n, d = check_r3()
    counts["R3"], details["R3"] = n, d
    n, d = check_r4()
    counts["R4"], details["R4"] = n, d
    n, d = check_r5()
    counts["R5"], details["R5"] = n, d
    n, d = check_r7()
    counts["R7"], details["R7"] = n, d
    n, d = check_r8()
    counts["R8"], details["R8"] = n, d
    n, d = check_r9()
    counts["R9"], details["R9"] = n, d
    n, d = check_r10()
    counts["R10"], details["R10"] = n, d
    return counts, details


# ---------------------------------------------------------------------- R9
def check_r9() -> tuple[int, list[str]]:
    """Duplicate lines inside one comment block = append-only editing.

    The review found four doc comments that had been written twice (the old
    text never deleted, the new text appended below). Duplicated comments are
    the visible symptom of "add, never clean up" and they rot fastest -- the
    stale copy is what a reader believes. Cheap to check, so it is checked.
    """
    hits: list[str] = []
    for f in go_files("internal") + go_files("cmd"):
        lines = code_lines(f)
        for i, line in enumerate(lines):
            if not line.startswith("//") or len(line.strip()) <= 5:
                continue
            j = i + 1
            block: list[str] = []
            while j < len(lines) and lines[j].startswith("//"):
                block.append(lines[j])
                j += 1
            if line in block:
                hits.append(f"{rel(f)}:{i + 1} {line.strip()[:60]}")
    return len(hits), hits


# --------------------------------------------------------------------- R10
AUDIT_FILE = "internal/services/audit.go"


def check_r10() -> tuple[int, list[str]]:
    """ActionLog rows may only be constructed in services/audit.go.

    P2 made audit a single writer. That is the property that stops the old
    two-track arrangement (handlers calling recordAudit, services calling
    RecordAction) from coming back one endpoint at a time: a new write path
    cannot invent its own audit insert, because the only insert lives in a
    function it cannot reach from the transport layer.

    Query sites are excluded: `Model(&models.ActionLog{})` and
    `[]models.ActionLog{}` build a type or a destination, not a row.
    """
    hits: list[str] = []
    for f in go_files("internal"):
        if rel(f) == AUDIT_FILE:
            continue
        for i, line in enumerate(code_lines(f), 1):
            if "models.ActionLog{" not in line:
                continue
            if "Model(&" in line or "[]models.ActionLog" in line:
                continue
            hits.append(f"{rel(f)}:{i} {line.strip()[:70]}")
    return len(hits), hits


LABELS = {
    "R1": "transport symbols (ResponseWriter/Request/ServeFile/Client)",
    "R1b": "files importing net/http only for status constants",
    "R2.files": f"files > {MAX_FILE_LINES} lines",
    "R2.funcs": f"functions > {MAX_FUNC_LINES} lines",
    "R3": "struct types in services pkg",
    "R4": "require* calls in handlers",
    "R5": "recordAudit calls in handlers",
    "R7": "add*Column clone functions in config pkg",
    "R8": "fetch() outside core/http.ts",
    "R9": "duplicated comment lines",
    "R10": "ActionLog writers outside services/audit.go",
}

# Observed, never enforced: these carry information, not a gate. R1b exists so
# the "domain still speaks in HTTP numbers" number stays visible without
# forcing a zero-gain rename of ~295 status constants.
OBSERVED = {"R1b"}


def main() -> int:
    args = set(sys.argv[1:])
    verbose = "--verbose" in args
    counts, details = collect()

    if "--baseline" in args or "--update" in args or not BASELINE.exists():
        existed = BASELINE.exists()
        BASELINE.write_text(
            json.dumps(counts, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
        )
        verb = "updated" if existed and "--update" in args else "written"
        print(f"baseline {verb}: {rel(BASELINE)}")
        for k in LABELS:
            print(f"  {k:10s} {counts[k]:5d}  {LABELS[k]}")
        if not existed and "--baseline" not in args and "--update" not in args:
            print("\n(no baseline existed -- created from current tree; re-run to enforce)")
            return 0
        return 0

    base: dict[str, int] = json.loads(BASELINE.read_text(encoding="utf-8"))
    grew: list[str] = []
    shrank: list[str] = []
    for k, label in LABELS.items():
        cur, was = counts[k], base.get(k, 0)
        if k in OBSERVED:
            print(f" i {k:10s} {cur:5d} / {was:5d}  {label}")
            continue
        flag = " "
        if cur > was:
            flag, grew = "!", grew + [f"{k}: {was} -> {cur}  {label}"]
        elif cur < was:
            flag, shrank = "+", shrank + [f"{k}: {was} -> {cur}  {label}"]
        print(f" {flag} {k:10s} {cur:5d} / {was:5d}  {label}")

    if verbose:
        for rule, items in details.items():
            if not items:
                continue
            print(f"\n--- {rule} ({len(items)}) ---")
            for it in items:
                print(f"  {it}")

    if grew:
        print("\nFAIL -- these counts must not grow (fix, or lower the baseline on purpose):")
        for g in grew:
            print(f"  {g}")
        print("\nOffenders: re-run with --verbose.")
        return 1

    if shrank:
        print("\nOK -- debt shrank. Lock the win in: python scripts/arch_check.py --update")
        for s in shrank:
            print(f"  {s}")
        return 0

    print("\nOK -- no architecture rule grew.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
