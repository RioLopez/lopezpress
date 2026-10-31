#!/usr/bin/env python3
"""truth_check.py -- 母文档声明 vs 仓库实测，逐项对账。

存在理由（第 95 轮）：
    项目失控的机制不是"文档写得不好"，而是**文档里混进了会变的状态**。
    静态事实（写了多少行代码）漂移很小；动态事实（多少文件未提交、
    库里多少脏数据、线上跑哪个版本）写进 md 的瞬间就开始过期。
    doc_check.py 只查份数/索引/横幅，不查数字，所以数字错了没人知道。

本脚本做两件事：
    1. DECLARED —— 从 dev/ai-dev/MASTER.md 的「事实基线」表抽出声明值，与实测比对，
       不一致 = DRIFT，退出码 1。
    2. LIVE —— 只报告不比对：未提交文件、库内状态分布、版本多源冲突。
       这些每天在变，不该写进文档，该每次现算。

口径唯一性：
    路由数复用 scripts/contract_check.py 的输出，文档数复用
    scripts/doc_check.py 的 collect_official()，不另立一套算法
    —— 否则"对账脚本"自己就成了第二个真相源。

Standard library only. No pip, no network.

Usage:
    python scripts/truth_check.py           # 全量
    python scripts/truth_check.py --declared # 只比对声明值（可进门禁）
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

try:  # Windows 控制台默认 cp936，中文输出会炸
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:  # pragma: no cover
    pass

ROOT = Path(__file__).resolve().parent.parent
MASTER = ROOT / "dev" / "ai-dev" / "MASTER.md"
SCRIPTS = ROOT / "scripts"

# 授权渗透测试留下的标题特征（R1–R7 各轮探针、注入、竞态、消毒器绕过…）。
# 启发式，不是判定：命中只说明"值得人工看一眼再删"。
PROBE_MARKERS = (
    "probe", "PROBE", "slug", "SLUG", "mxss", "hijack", "trash", "race",
    "sanitiz", "pin-verify", "BIGTITLE", "COL", "XSS", "xss",
    "R1", "R2", "R3", "R4", "R5", "R6", "R7", "aaa", "bbb",
)

# 首启种子示例与手工占位（这些不是探针，但同样不是内容）。
PLACEHOLDER_MARKERS = (
    "欢迎来到内容平台", "Go 后端最佳实践", "前后端分离架构",
    "这是一个", "封面测试", "浏览器自动化测试",
)

ONLY_DECLARED = "--declared" in sys.argv

drift: list[str] = []


def say(line: str = "") -> None:
    print(line)


def drift_found(key: str, declared: object, actual: object) -> None:
    drift.append(f"{key}: 声明 {declared} / 实测 {actual}")
    print(f"  [DRIFT] {key:<16} 声明 {declared!s:<12} 实测 {actual}")


def ok(key: str, value: object) -> None:
    print(f"  [ok]    {key:<16} {value}")


# ---------------------------------------------------------------- 声明解析

def table_row(label: str, hint: str = "") -> str:
    """取 MASTER.md 里 `| label | ... |` 那行的合并单元格文本。

    hint 用于消歧义：同一标签在多张表里出现（比如表格里也有「文档」列），
    只取内容同时命中 hint 的那一行。
    """
    if not MASTER.exists():
        return ""
    text = MASTER.read_text(encoding="utf-8", errors="ignore")
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped.startswith("|"):
            continue
        cells = [c.strip() for c in stripped.strip("|").split("|")]
        if cells and cells[0].strip("* ") == label:
            body = " ".join(cells[1:])
            if hint and not re.search(hint, body):
                continue
            return body
    return ""


def declared_number(row: str, pattern: str) -> int | None:
    m = re.search(pattern, row)
    if not m:
        return None
    return int(m.group(1).replace(",", ""))


# ---------------------------------------------------------------- 实测

def _files(*patterns: tuple[str, str]) -> list[Path]:
    out: list[Path] = []
    for base, glob in patterns:
        root = ROOT / base
        if root.exists():
            out.extend(root.rglob(glob))
    return out


def _line_count(paths: list[Path]) -> int:
    """按换行符计数，与 `wc -l` 同口径（母文档的数字就是这么来的）。"""
    total = 0
    for p in paths:
        try:
            total += p.read_text(encoding="utf-8", errors="ignore").count("\n")
        except OSError:
            pass
    return total


def actual_go_files() -> int:
    return len(_files(("internal", "*.go"), ("cmd", "*.go")))


def actual_go_lines() -> int:
    return _line_count(_files(("internal", "*.go"), ("cmd", "*.go")))


def actual_test_files() -> int:
    """按 *_test.go 计文件数（含只有 helper、没有 Test 函数的 testutil_test.go），
    与母文档口径一致；不要用"含 ^func Test 的文件数"，那会少算 1。"""
    return len(_files(("internal", "*_test.go")))


def actual_test_funcs() -> int:
    return sum(len(re.findall(r"^func Test", p.read_text(encoding="utf-8", errors="ignore"), re.M))
               for p in _files(("internal", "*.go")))


def actual_fe_files() -> int:
    return len(_files(("web/src", "*.ts"), ("web/src", "*.tsx")))


def actual_fe_lines() -> int:
    return _line_count(_files(("web/src", "*.ts"), ("web/src", "*.tsx")))


def actual_fe_pages() -> int:
    pages = ROOT / "web" / "src" / "pages"
    return len(list(pages.glob("*.tsx"))) if pages.exists() else 0


def actual_routes() -> int | None:
    """复用 contract_check.py 的口径，不自己数 router.go。"""
    try:
        r = subprocess.run([sys.executable, str(SCRIPTS / "contract_check.py")],
                           cwd=ROOT, capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.SubprocessError):
        return None
    m = re.search(r"后端路由\s*(\d+)\s*个", r.stdout)
    return int(m.group(1)) if m else None


def actual_docs() -> int | None:
    """复用 doc_check.py 的 collect_official()，口径唯一。"""
    try:
        sys.path.insert(0, str(SCRIPTS))
        import doc_check  # noqa: PLC0415
        return len(doc_check.collect_official())
    except Exception:
        return None


def actual_version() -> str:
    f = ROOT / "VERSION"
    return f.read_text(encoding="utf-8").strip() if f.exists() else "?"


# ---------------------------------------------------------------- 1. 声明对账

def check_declared() -> None:
    say("== 1. 母文档声明 vs 实测 ==")
    if not MASTER.exists():
        say("  [FAIL] 找不到 dev/ai-dev/MASTER.md")
        drift.append("MASTER.md 缺失")
        return

    back = table_row("后端规模")
    front = table_row("前端规模")
    tests = table_row("测试")
    routes = table_row("路由")
    version = table_row("版本")
    docs = table_row("文档", hint=r"份")

    rows: list[tuple[str, int | None, int | None]] = [
        ("后端 .go 文件", declared_number(back, r"(\d+)\s*个\s*`\.go`"), actual_go_files()),
        ("后端行数", declared_number(back, r"([\d,]+)\s*行"), actual_go_lines()),
        ("前端文件", declared_number(front, r"(\d+)\s*个\s*`\.ts\(x\)`"), actual_fe_files()),
        ("前端行数", declared_number(front, r"([\d,]+)\s*行"), actual_fe_lines()),
        ("前端页面", declared_number(front, r"(\d+)\s*页面"), actual_fe_pages()),
        ("测试文件", declared_number(tests, r"(\d+)\s*个文件"), actual_test_files()),
        ("测试用例", declared_number(tests, r"\*\*(\d+)\*\*\s*个\s*`Test\*`"), actual_test_funcs()),
        ("后端路由", declared_number(routes, r"后端\s*\*\*(\d+)\*\*\s*条"), actual_routes()),
        ("官方文档数", declared_number(docs, r"官方\s*(\d+)\s*份"), actual_docs()),
    ]

    for key, dec, act in rows:
        if dec is None:
            say(f"  [skip]  {key:<16} 母文档里没解析到声明值（表格格式变了？）")
            drift.append(f"{key}: 声明值无法解析")
        elif act is None:
            say(f"  [skip]  {key:<16} 实测失败（依赖脚本跑不通）")
            drift.append(f"{key}: 实测失败")
        elif dec != act:
            drift_found(key, dec, act)
        else:
            ok(key, act)

    dec_ver = re.search(r"`VERSION`\s*=\s*\*\*`([\d.]+)`\*\*", version)
    if dec_ver:
        if dec_ver.group(1) != actual_version():
            drift_found("版本", dec_ver.group(1), actual_version())
        else:
            ok("版本", actual_version())
    else:
        say("  [skip]  版本             母文档里没解析到")
        drift.append("版本: 声明值无法解析")


# ---------------------------------------------------------------- 2. 实时状态

def git(*args: str) -> str:
    try:
        r = subprocess.run(["git", *args], cwd=ROOT, capture_output=True,
                           text=True, timeout=60)
        return r.stdout.strip()
    except (OSError, subprocess.SubprocessError):
        return ""


def report_live() -> None:
    say()
    say("== 2. 实时状态（不比对，只报告；这些值不该写进文档）==")

    status = git("status", "--porcelain")
    files = [ln[3:] for ln in status.splitlines() if ln.strip()]
    say(f"  未提交文件      {len(files)}")
    dirs: dict[str, int] = {}
    for f in files:
        d = str(Path(f).parent).replace("\\", "/")
        dirs[d] = dirs.get(d, 0) + 1
    for d, n in sorted(dirs.items(), key=lambda kv: -kv[1])[:6]:
        say(f"    {n:>4}  {d}/")

    last = git("log", "-1", "--format=%h %ad %s", "--date=short")
    say(f"  最后提交        {last or '?'}")

    # 版本多源：VERSION 是唯一真相，其他文件提到"应用版本"就必须一致。
    # 注意别把 Go 工具链版本（go 1.26.10）和示例 IP（1.2.3.4）当成版本源 ——
    # 第一版脚本就是这么误报的，只认显式的发布版本写法。
    ver = actual_version()
    say(f"  VERSION(真相源) {ver}")
    ver_pat = re.compile(
        r"(?:\bAPP_VERSION\b|\bRELEASE\b|\bVERSION\b)\s*[=:]\s*[\"']?(1\.\d+\.\d+)"
        r"|\blopezpress-(1\.\d+\.\d+)")
    stale: list[str] = []
    targets: list[Path] = []
    for base in ("deploy", "scripts"):
        root = ROOT / base
        if root.exists():
            targets.extend(p for p in root.rglob("*")
                           if p.is_file() and p.suffix in {".sh", ".bat", ".ps1", ".py"})
    targets.extend(p for p in ROOT.glob("*.bat"))
    for p in targets:
        try:
            text = p.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        for m in ver_pat.finditer(text):
            found = m.group(1) or m.group(2)
            if found and found != ver:
                stale.append(f"{p.relative_to(ROOT).as_posix()}: {found}")
    if stale:
        say(f"  [版本冲突] {len(stale)} 处 ≠ VERSION（版本多源 = 部署事故温床）")
        for s in stale[:8]:
            say(f"    - {s}")
    else:
        say("  [版本冲突] 无")


def report_db() -> None:
    db = ROOT / "data" / "app.db"
    if not db.exists():
        say("  DB              data/app.db 不存在，跳过")
        return
    try:
        import sqlite3
        con = sqlite3.connect(f"file:{db.as_posix()}?mode=ro", uri=True)
        c = con.cursor()
        mb = lambda n: f"{n / 1048576:.1f} MB"  # noqa: E731
        say(f"  DB 体积         {mb(db.stat().st_size)}")
        try:
            for name, size in c.execute(
                    "select name, sum(pgsize) from dbstat group by name "
                    "order by 2 desc limit 3"):
                say(f"    {name:<16} {mb(size or 0)}")
        except sqlite3.DatabaseError:
            pass
        parts = list(c.execute("select status, count(*) from posts group by status order by 2 desc"))
        say("  文章状态        " + "  ".join(f"{s}={n}" for s, n in parts))
        # 探针残留 + 占位内容：授权渗透测试留下的探针，和首启种子/手工占位。
        # 两者都不是能见人的文章，但成因不同，分开计数。
        # 注意标题可能有 1 MB，只取前 120 字符做匹配。
        probe: dict[str, int] = {}
        placeholder = 0
        rest = 0
        total = 0
        for title in c.execute("select title from posts"):
            t = (title[0] or "")[:120]
            total += 1
            if any(mark in t for mark in PROBE_MARKERS):
                probe["hit"] = probe.get("hit", 0) + 1
            elif any(mark in t for mark in PLACEHOLDER_MARKERS):
                placeholder += 1
            elif len(t.strip()) <= 2 or t.strip().isdigit():
                placeholder += 1  # "1" "2" "7" 这类手工占位
            else:
                rest += 1
        hit = probe.get("hit", 0)
        say(f"  探针残留        {hit} / {total} 篇")
        say(f"  种子/占位       {placeholder} / {total} 篇")
        say(f"  其余(需人工判)  {rest} / {total} 篇")
        con.close()
    except Exception as e:  # noqa: BLE001
        say(f"  DB 读取失败     {e}")


def main() -> int:
    say("truth_check -- 母文档声明 vs 仓库实测")
    say("-" * 64)
    check_declared()

    if not ONLY_DECLARED:
        report_live()
        report_db()

    say()
    say("-" * 64)
    if drift:
        say(f"结论：{len(drift)} 项失真 —— 母文档不能当事实用，先修它再干活。")
        for d in drift:
            say(f"  - {d}")
        return 1
    say("结论：母文档声明与实测一致。")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
