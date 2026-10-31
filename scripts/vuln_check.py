#!/usr/bin/env python3
"""Dependency vulnerability gate for lopezpress.

Why this exists
---------------
`docs/SECURITY_BASELINE.md` 里「依赖漏洞扫描」这一项，曾是最没人能收尾的阻塞项：
"govulncheck needs a newer Go and a working checksum database, so
it is not installed". The item sat there for three rounds because "not
installed" reads like "not applicable", and meanwhile the build shipped a Go
1.22.12 standard library with 46 reachable advisories — including a WebP decode
panic on a code path this project calls for every uploaded image, and a memory
amplification in JWT header parsing on the login path.

The point of this file is that the finding should not depend on remembering to
run the tool. Run it before a release, and run it after touching go.mod.

What it checks
--------------
  Go      govulncheck ./...   → reachable vulnerabilities must be 0
  npm     npm audit --omit=dev → production dependencies in web/

Exit codes
  0  clean
  1  reachable vulnerabilities found (block the release)
  2  a scanner is missing or could not run (fix the environment, not the code)

Notes
-----
* `GOSUMDB=off` (set on this machine) blocks the Go toolchain download that a
  modern `go` directive triggers. Export `GOSUMDB=sum.golang.org` first, or the
  scan fails with "checksum database disabled" rather than a real result.
* Only *reachable* counts are enforced. govulncheck also lists advisories in
  modules that are required but whose vulnerable symbols our code never calls;
  failing on those would make the gate impossible to keep green and would train
  everyone to ignore it.
"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
GO_DEPS = {"module": "lopezpress"}


def find_govulncheck() -> str | None:
    exe = shutil.which("govulncheck")
    if exe:
        return exe
    # `go install` puts it in GOBIN, else GOPATH/bin — neither is always on PATH.
    for candidate in (
        os.environ.get("GOBIN", ""),
        os.path.join(os.environ.get("GOPATH", ""), "bin"),
        str(Path.home() / "go" / "bin"),
    ):
        if not candidate:
            continue
        for name in ("govulncheck", "govulncheck.exe"):
            p = Path(candidate) / name
            if p.is_file():
                return str(p)
    return None


def go_on_path(env: dict[str, str]) -> dict[str, str] | None:
    """Return env with a usable `go` on PATH, or None if there is none.

    govulncheck shells out to `go env GOMOD` and `go list`. With no `go`
    reachable it cannot resolve the module, and it reports that as "no go.mod
    file" -- a message that sends the reader to inspect go.mod, which is
    usually sitting right there. Getting a real answer means finding the
    toolchain first, since a non-login shell does not always carry it.
    """
    if shutil.which("go", path=env.get("PATH")):
        return env
    for candidate in (
        os.environ.get("GOROOT", ""),
        str(Path.home() / "go-sdk" / "go" / "bin"),
        str(Path.home() / "sdk" / "go" / "bin"),
        str(Path.home() / "go" / "bin"),
        r"C:\Program Files\Go\bin",
        "/usr/local/go/bin",
        "/usr/lib/go/bin",
    ):
        if not candidate:
            continue
        if (Path(candidate) / "go.exe").is_file() or (Path(candidate) / "go").is_file():
            return dict(env, PATH=candidate + os.pathsep + env.get("PATH", ""))
    return None


def run(cmd: list[str], cwd: Path) -> tuple[int, str]:
    env = go_on_path(dict(os.environ))
    if env is None:
        return 126, "no go toolchain on PATH"
    # This machine ships with GOSUMDB=off. govulncheck drives `go list`
    # internally, so with the checksum database disabled its toolchain lookup
    # fails and it reports the misleading "no go.mod file" -- the module is
    # right there. Pin the public sumdb for the subprocess: the scan then
    # measures the dependency graph instead of the shell's environment.
    env["GOSUMDB"] = "sum.golang.org"
    # `go` cannot start without a build cache either, and this shell has no
    # %LocalAppData% to derive one from. Give the subprocess a cache whenever
    # the environment has none.
    if not env.get("GOCACHE"):
        cache = Path.home() / ".cache" / "go-build"
        cache.mkdir(parents=True, exist_ok=True)
        env["GOCACHE"] = str(cache)
    try:
        r = subprocess.run(cmd, cwd=str(cwd), env=env,
                           capture_output=True, text=True, timeout=900)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return 127, str(exc)
    return r.returncode, (r.stdout or "") + (r.stderr or "")


def check_go() -> int:
    exe = find_govulncheck()
    if not exe:
        print("  SKIP  govulncheck not installed. Fix with:")
        print("          GOSUMDB=sum.golang.org go install golang.org/x/vuln/cmd/govulncheck@latest")
        return 2

    code, out = run([exe, "./..."], ROOT)
    if code == 126:
        print("  FAIL  no Go toolchain on PATH, so govulncheck cannot resolve the module.")
        print("        Add the directory holding the go binary (e.g. %GOROOT%\\bin) to PATH.")
        return 2
    if code == 127:
        print(f"  FAIL  could not run govulncheck: {out.strip()[:200]}")
        return 2
    if "checksum database disabled" in out:
        print("  FAIL  Go toolchain download blocked by GOSUMDB=off.")
        print("        Re-run with GOSUMDB=sum.golang.org in the environment.")
        return 2

    m = re.search(r"Your code is affected by (\d+) vulnerabilit", out)
    if not m:
        # No phrase at all usually means the packages had no findings at all, or
        # the tool changed its wording; show the tail so a silent pass is visible.
        print("  WARN  could not parse a count from govulncheck output; tail follows:")
        for line in out.strip().splitlines()[-6:]:
            print(f"        {line}")
        return 2
    reached = int(m.group(1))
    if reached == 0:
        print("  ok    0 reachable vulnerabilities in the Go module graph")
        return 0

    print(f"  FAIL  {reached} reachable vulnerabilit{'y' if reached == 1 else 'ies'}")
    # Print the headline of each finding: enough to decide, not a wall of traces.
    for block in out.split("Vulnerability #")[1:]:
        lines = [l.strip() for l in block.strip().splitlines() if l.strip()]
        if not lines:
            continue
        adv_match = re.search(r"GO-\d{4}-\d+", lines[0])
        adv_id = adv_match.group(0) if adv_match else lines[0]
        title = next(
            (l for l in lines[1:] if not l.startswith(("More info", "Module", "Found in", "Fixed in", "Example"))),
            "",
        )
        fixed = next((l for l in lines if l.startswith("Fixed in:")), "")
        print(f"        - {adv_id}  {title}")
        if fixed:
            print(f"          {fixed}")
    return 1


def check_npm() -> int:
    web = ROOT / "web"
    if not (web / "package.json").is_file():
        return 0
    npm = shutil.which("npm") or shutil.which("npm.cmd")
    if not npm:
        print("  SKIP  npm not installed")
        return 2
    code, out = run([npm, "audit", "--omit=dev", "--json"], web)
    # npm audit exits non-zero when it finds anything; the JSON tells us severity.
    try:
        import json

        data = json.loads(out[out.index("{"):])
        meta = data.get("metadata", {}).get("vulnerabilities", {})
        total = sum(int(v) for k, v in meta.items() if k in ("low", "moderate", "high", "critical"))
    except Exception:
        if code == 127:
            return 2
        print("  WARN  could not parse npm audit output (no network?)")
        return 2
    if total == 0:
        print("  ok    0 production vulnerabilities in web/")
        return 0
    print(f"  FAIL  {total} production vulnerabilities in web/")
    for name, adv in (data.get("vulnerabilities") or {}).items():
        if adv.get("severity") in ("high", "critical"):
            print(f"        - {name}: {adv.get('severity')} (via {', '.join(adv.get('via', [])[:2])})")
    return 1


def main() -> int:
    print("== dependency vulnerability gate ==")
    worst = 0
    worst = max(worst, check_go())
    worst = max(worst, check_npm())
    print()
    if worst == 0:
        print("PASS -- no known reachable vulnerabilities.")
    elif worst == 1:
        print("FAIL -- upgrade the dependency or the toolchain before releasing.")
        print("       A Go standard-library finding means bumping the `go` line in go.mod;")
        print("       Go 1.21+ downloads the required toolchain automatically.")
    else:
        print("INCOMPLETE -- a scanner is missing, so this run proves nothing.")
    return worst


if __name__ == "__main__":
    sys.exit(main())
