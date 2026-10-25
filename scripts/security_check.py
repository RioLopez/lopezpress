#!/usr/bin/env python3
"""Static security gate for lopezpress.

Why this exists
---------------
Three rounds of external pentesting found the same class of bug repeatedly, and
each time the fix covered only the instance the report named:

  * R3-05 capped the login body — 18 other handlers kept decoding an unbounded
    body.
  * R4-10 reported "the importer has no name limit" — the tag path, the category
    CRUD endpoints and the cover/profile/review-note fields all had none either.
  * R1-01 reported four endpoints bypassing public_read — the cause was a route
    table annotation, not the handlers.

The lesson is not "be more careful next time". It is that a security property
which lives only in a reviewer's attention will regress. This file turns the
cheap, mechanical half of that review into something a machine runs, so the
answer to "did we break it again" does not depend on someone remembering.

What it checks (all heuristic, all with a stated reason):
  S1  every route declares an auth mode that matches its path shape
  S2  no handler decodes a body without a ceiling somewhere above it
  S3  service-layer writes that target a row by id carry a state precondition
  S4  every mutating handler reaches an audit write
  S5  user-writable string fields have a declared ceiling
  S6  no raw string concatenation into SQL
  S7  no user input reaching template.HTML / unescaped output helpers
  S8  secrets are never logged
  S9  a handler's own MaxBytesReader never exceeds the route's declared ceiling

Exit code 0 = no new offenders. Non-zero = the count grew past the baseline in
scripts/security_baseline.json (a ratchet: existing debt is recorded, new debt
fails the build).
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASELINE = os.path.join(ROOT, "scripts", "security_baseline.json")


def read(path):
    with open(os.path.join(ROOT, path), encoding="utf-8", errors="replace") as f:
        return f.read()


def walk(rel_root, suffix=".go"):
    out = []
    base = os.path.join(ROOT, rel_root)
    for dirpath, _dirnames, filenames in os.walk(base):
        for fn in filenames:
            if fn.endswith(suffix):
                full = os.path.join(dirpath, fn)
                out.append(os.path.relpath(full, ROOT).replace("\\", "/"))
    return sorted(out)


findings = []  # (rule, file, detail)


def flag(rule, path, detail):
    findings.append((rule, path, detail))


# --- S1: route table sanity -------------------------------------------------
def check_routes():
    src = read("internal/router/router.go")
    # Pull each route literal.
    for m in re.finditer(r'\{method:\s*"(\w+)",\s*path:\s*"([^"]+)"([^}]*)\}', src):
        method, path, rest = m.group(1), m.group(2), m.group(3)
        auth = re.search(r'auth:\s*(\w+)', rest)
        auth = auth.group(1) if auth else "<none>"
        roles = "roles:" in rest
        # A write endpoint must never be authPublic: anonymous callers cannot
        # legitimately create anything, and the failure mode (an unauthenticated
        # state change) is the worst one this table can produce.
        if method in ("POST", "PUT", "PATCH", "DELETE") and auth == "authPublic":
            if path not in ("/api/auth/login", "/api/auth/register",
                            "/api/auth/email-code", "/api/auth/password/reset",
                            "/api/auth/password/forgot"):
                flag("S1", "internal/router/router.go",
                     f"{method} {path} is authPublic — a write path open to anonymous callers")
        # A path under /api/admin must never be reachable without a role list.
        if path.startswith("/api/admin") and not roles:
            flag("S1", "internal/router/router.go",
                 f"{method} {path} has no roles guard despite the /api/admin prefix")
        # authOptional is for reads only; on a write it means "authenticate if
        # convenient", which silently accepts anonymous writes.
        # The view counter is a POST that only increments a number; anonymous
        # readers must reach it or the metric is wrong. It is a read with a
        # side effect, not a write endpoint.
        if method in ("POST", "PUT", "PATCH", "DELETE") and auth == "authOptional" \
                and not path.endswith("/view"):
            flag("S1", "internal/router/router.go",
                 f"{method} {path} is authOptional — writes must be authRequired")


# --- S2: body ceiling coverage ---------------------------------------------
def check_body_limits():
    # The router applies defaultBodyLimit to every route, so a handler is only
    # exposed if it reads r.Body directly OUTSIDE the route table (e.g. a
    # mux.HandleFunc added by hand) or bypasses http.MaxBytesReader on a
    # multipart path.
    router = read("internal/router/router.go")
    if "limitBody(limit, h)" not in router:
        flag("S2", "internal/router/router.go",
             "register() no longer applies limitBody — every route lost its ceiling")
    if "defaultBodyLimit" not in router:
        flag("S2", "internal/router/router.go", "defaultBodyLimit is gone")
    # Any bypass of the table?
    for path in walk("internal"):
        if path.endswith("_test.go"):
            continue
        src = read(path)
        if "mux.HandleFunc" in src and path != "internal/router/router.go":
            flag("S2", path, "registers a route outside the table, bypassing limitBody")


# --- S9: a handler's body cap must not exceed its route's --------------------
#
# The route table runs first, so whichever ceiling is SMALLER wins and the other
# becomes dead code. That is not symmetric in its consequences:
#
#   * handler > route  → the handler's documented limit is fiction. This is how
#     the backup-import route ended up capped at the 1 MiB default while its
#     handler promised 20 MiB, i.e. an operator could export a 2 MiB backup and
#     be unable to restore it. Data loss wearing a 413.
#   * handler < route  → fine and intentional: the route stops the buffering,
#     the handler applies a policy (the operator's upload_max_image, say).
#
# So only the first direction is flagged. Both sides must be resolvable to a
# number; a handler using a runtime variable is skipped rather than guessed at.
_NUM_EXPR = re.compile(r"^[\d\s*+<>]+$")


def _eval_bytes(expr):
    expr = expr.strip()
    if _NUM_EXPR.match(expr):
        try:
            return int(eval(expr, {"__builtins__": {}}, {}))  # noqa: S307 - digits only
        except Exception:
            return None
    return None


def _route_limits():
    """path -> (effective ceiling in bytes, handler name), from the route table."""
    src = read("internal/router/router.go") + read("internal/router/bodylimit.go")
    consts = {}
    for m in re.finditer(r"^const\s+(\w+)\s*=\s*([^\n/]+)", src, re.M):
        v = _eval_bytes(m.group(2))
        if v is not None:
            consts[m.group(1)] = v
    out = {}
    for m in re.finditer(
            r'path:\s*"([^"]+)"[^}]*?handler:\s*handlers\.(\w+)', src):
        path, handler = m.group(1), m.group(2)
        body = m.group(0)
        mb = re.search(r"maxBody:\s*(\w+)", body)
        limit = consts.get(mb.group(1)) if mb else consts.get("defaultBodyLimit")
        if limit is not None:
            out[path] = (limit, handler)
    return out


def check_body_limit_vs_handler():
    limits = _route_limits()
    if not limits:
        flag("S9", "internal/router/router.go",
             "could not parse the route table's body ceilings — the check is blind")
        return
    for path in walk("internal/handlers"):
        if path.endswith("_test.go"):
            continue
        src = read(path)
        # Each func: find its body, then the caps it sets.
        for fm in re.finditer(r"^func (\w+)\(", src, re.M):
            name = fm.group(1)
            start = fm.end()
            nxt = src.find("\nfunc ", start)
            body = src[start:nxt if nxt != -1 else len(src)]
            for cm in re.finditer(
                    r"MaxBytesReader\(w,\s*r\.Body,\s*([^)]+)\)", body):
                got = _eval_bytes(cm.group(1))
                if got is None:
                    continue  # runtime value: the route is the real ceiling
                for rpath, (limit, hname) in limits.items():
                    if hname != name:
                        continue
                    if got > limit:
                        flag("S9", path,
                             f"{name} caps its body at {got} but route {rpath} "
                             f"caps at {limit}; the route runs first, so the "
                             f"handler's limit is dead code — declare "
                             f"maxBody on the route instead")


# --- S3: state transitions must be conditional ------------------------------
def check_transitions():
    # An UPDATE gated only on id can move a row in ANY state: approve racing
    # delete re-published a post the author had removed (R4-11). The sanctioned
    # path is transitionPost / transitionPostTx.
    for path in walk("internal/services"):
        if path.endswith("_test.go") or path.endswith("transition.go"):
            continue
        src = read(path)
        for m in re.finditer(r'Model\(&models\.Post\{\}\)\.Where\("id = \?"[^)]*\)[^\n]*\n?(?:[^\n]*\n){0,12}?\s*Updates\(([^)]*)\)', src):
            body = m.group(1)
            # Only a STATUS change needs a state precondition. Updating the
            # title/body of a row identified by id is exactly what a content
            # edit is, and requiring a status in that WHERE would break it.
            if '"status"' not in body and "status" not in body:
                continue
            flag("S3", path,
                 "status UPDATE gated only on id — use transitionPost so the expected state is part of the WHERE")


# --- S4: audit coverage for mutations ---------------------------------------
def check_audit():
    # Every handler wired to a mutating route should reach s.audit somewhere in
    # the service call it makes. This is a coarse check: it counts audit calls
    # in the services package against mutating handlers, and reports a ratio
    # regression rather than trying to trace call graphs.
    router = read("internal/router/router.go")
    mutations = re.findall(r'\{method:\s*"(POST|PUT|PATCH|DELETE)",\s*path:\s*"([^"]+)"', router)
    # Endpoints that legitimately do not write an audit row: pure reads behind
    # POST (logout is client-side), the read-only index, and login (its own
    # login_logs trail).
    exempt = {
        "/api/auth/login", "/api/auth/logout", "/api/auth/logout-all",
        "/api/auth/email-code", "/api/auth/password/reset",
        "/api/auth/password/forgot", "/api/auth/register",
        "/api/auth/change-password", "/api/auth/verify-email",
        "/api/posts/{id}/view", "/api/auth/me",
    }
    audit_calls = sum(read(p).count("s.audit(") for p in walk("internal/services"))
    if audit_calls < 39:  # ratchet floor: today's count, may only rise
        flag("S4", "internal/services",
             f"only {audit_calls} s.audit() call sites remain — audit coverage regressed")
    if len([m for m in mutations if m[1] not in exempt]) < 20:
        flag("S4", "internal/router/router.go",
             "mutating route count dropped unusually — did the table get truncated?")


# --- S5: user-writable string fields need a ceiling -------------------------
def check_field_ceilings():
    # The DTOs are the complete list of what a client can write. Each string
    # field should be either bounded (a Max*Len constant), validated by a
    # Validate* helper, or explicitly exempt (ids, enums, secrets with their own
    # rules).
    dto = read("internal/services/dto.go")
    # Only *Input types describe what a CLIENT may write; the response types
    # in the same file are server-produced and legitimately unbounded.
    fields = []
    for im in re.finditer(r'type (\w*Input) struct \{(.*?)\n\}', dto, re.S):
        for fm in re.finditer(r'^\t(\w+)\s+string\s+`json:"([^"]+)"', im.group(2), re.M):
            fields.append(fm.groups())
    limits = read("internal/services/limits.go")
    # Fields whose length is enforced elsewhere or is inherently bounded.
    exempt = {
        "username": "isValidUsername", "email": "isValidEmail",
        "password": "password policy", "currentpassword": "password policy",
        "newpassword": "password policy", "code": "fixed-length code",
        "purpose": "enum", "role": "enum", "id": "primary key",
        "confirm": "fixed phrase", "authormode": "enum", "kind": "enum",
        "img_host_mode": "enum", "snapshotfile": "server-generated name",
        "target": "enum", "action": "enum", "decision": "enum",
        "status": "enum", "sortby": "enum", "order": "enum",
        # Slug is bounded by Slugify() (slugMaxRunes = 80), which every write
        # path passes through before the value reaches the row.
        "slug": "bounded by Slugify",
    }
    unbounded = []
    for name, json_name in fields:
        key = name.lower()
        if key in exempt:
            continue
        # Is it mentioned in limits.go or validated in the service that writes it?
        if name in limits or name in limits:
            continue
        unbounded.append(f"{name} ({json_name})")
    # Allow a handful of remaining known-bounded fields.
    known_ok = {"Tags", "CategoryID", "Pinned", "Page", "Size", "DryRun",
                "KeepTokens", "PasswordHashes"}
    unbounded = [u for u in unbounded if u.split(" ")[0] not in known_ok]
    if len(unbounded) > 0:
        flag("S5", "internal/services/dto.go",
             "string field(s) with no visible ceiling: " + ", ".join(sorted(unbounded)))


# --- S6: SQL injection surface ---------------------------------------------
def check_sql():
    for path in walk("internal"):
        if path.endswith("_test.go"):
            continue
        src = read(path)
        # Raw/Exec with string concatenation is the injection shape. Formatted
        # placeholders (?) are fine; a `+` or Sprintf into the query is not.
        for m in re.finditer(r'\.(Raw|Exec)\(\s*(fmt\.Sprintf\([^)]*\)|[^"`\n]*\+[^"`\n]*)', src):
            snippet = m.group(0)[:90].replace("\n", " ")
            flag("S6", path, f"SQL built by concatenation: {snippet}")


# --- S7: unescaped output --------------------------------------------------
def check_output():
    for path in walk("internal"):
        if path.endswith("_test.go"):
            continue
        src = read(path)
        if "template.HTML(" in src:
            flag("S7", path, "template.HTML() bypasses escaping — verify every input is trusted")


# --- S8: secrets in logs ---------------------------------------------------
SECRET_HINTS = ("password", "passwd", "secret", "token", "apikey", "api_key",
                "privatekey", "private_key")
def check_logs():
    for path in walk("internal"):
        if path.endswith("_test.go"):
            continue
        src = read(path)
        for m in re.finditer(r'(log\.\w+|slog\.\w+)\(([^\n]*)', src):
            line = m.group(0)
            low = line.lower()
            for hint in SECRET_HINTS:
                if hint in low and "err" not in low and "failed" not in low:
                    # Logging an error that may mention a secret is tolerated;
                    # logging a value named like a secret is not.
                    if re.search(r'\b\w*' + hint + r'\w*\b', low):
                        flag("S8", path, f"log line mentions {hint}: {line[:90]}")
                        break


def main():
    check_routes()
    check_body_limits()
    check_body_limit_vs_handler()
    check_transitions()
    check_audit()
    check_field_ceilings()
    check_sql()
    check_output()
    check_logs()

    # Ratchet: compare counts per rule against the recorded baseline.
    base = {}
    if os.path.exists(BASELINE):
        with open(BASELINE, encoding="utf-8") as f:
            base = json.load(f).get("counts", {})
    counts = {}
    for rule, _path, _detail in findings:
        counts[rule] = counts.get(rule, 0) + 1

    print("security_check: static heuristics")
    print(f"  {'rule':6} {'now':>4} {'baseline':>9}")
    grew = []
    for rule in sorted(set(list(counts) + list(base))):
        now = counts.get(rule, 0)
        was = base.get(rule, 0)
        mark = ""
        if now > was:
            mark = "  <-- grew"
            grew.append(rule)
        print(f"  {rule:6} {now:4d} {was:9d}{mark}")

    if findings:
        print("\n  current findings:")
        for rule, path, detail in findings:
            print(f"    [{rule}] {path}: {detail}")

    if grew:
        print("\nFAIL -- these counts must not grow (fix, or lower the baseline on purpose):")
        for r in grew:
            print(f"  {r}: {base.get(r, 0)} -> {counts[r]}")
        return 1
    print("\nOK -- no security rule grew.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
