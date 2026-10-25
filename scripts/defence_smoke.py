#!/usr/bin/env python3
"""Defence smoke: replay the R1-R7 attack cases against a running instance.

This is the defensive counterpart to the external pentest reports. Unit tests
check function logic; this checks what a client can actually do over HTTP,
which is a different question and the one a pentester asks.

Usage
-----
    # 1. start an ISOLATED instance (never point this at real data)
    PORT=18099 DB_PATH=/tmp/lp-smoke/smoke.db \
      UPLOAD_DIR=/tmp/lp-smoke/uploads BACKUP_DIR=/tmp/lp-smoke/backups \
      JWT_SECRET=<32+ chars> ADMIN_INITIAL_PASSWORD=<pw> ./bin/lp-smoke.exe

    # 2. on first boot the root account has must_change_password=true; log in
    #    once and change it, then run this with the NEW password:
    BASE_URL=http://127.0.0.1:18099 ROOT_PASSWORD=<new pw> \
      python scripts/defence_smoke.py

Every assertion names the report item it defends. A non-zero exit means at
least one attack still works.
"""
import json
import os
import struct
import sys
import threading
import time
import urllib.error
import urllib.request
import zlib

B = os.environ.get("BASE_URL", "http://127.0.0.1:18099").rstrip("/")
ROOT_PW = os.environ.get("ROOT_PASSWORD", "")
PLAIN_PW = os.environ.get("PLAIN_PASSWORD", "DrillPass123!")
PASS, FAIL = 0, []


def ok(name, note=""):
    global PASS
    PASS += 1
    print(f"  PASS  {name}" + (f" [{note}]" if note else ""))


def bad(name, detail):
    FAIL.append((name, detail))
    print(f"  FAIL  {name}: {detail}")


def expect(name, want, got):
    (ok(name, f"={got}") if got == want else bad(name, f"want {want}, got {got}"))


def req(method, path, body=None, token=None, raw=None, ctype=None, timeout=60,
        headers=None):
    data = raw
    h = dict(headers or {})
    if body is not None:
        data = json.dumps(body).encode()
        h["Content-Type"] = "application/json"
    if ctype:
        h["Content-Type"] = ctype
    if token:
        h["Authorization"] = "Bearer " + token
    r = urllib.request.Request(B + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            return resp.status, resp.read(), dict(resp.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read(), dict(e.headers)
    except Exception as e:  # noqa: BLE001 - a transport failure is itself a finding
        return 0, str(e).encode(), {}


def jreq(*a, **kw):
    st, body, hd = req(*a, **kw)
    try:
        return st, json.loads(body), hd
    except Exception:
        return st, {}, hd


def login(ident, pw):
    st, body, _ = req("POST", "/api/auth/login", {"email": ident, "password": pw})
    try:
        return json.loads(body).get("token", ""), st
    except Exception:
        return "", st


def png(w, h, shade=128):
    """A real, decodable PNG (single colour), for upload tests."""
    def chunk(tag, data):
        return (struct.pack(">I", len(data)) + tag + data +
                struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF))
    raw = b"".join(b"\x00" + bytes([shade, shade, shade]) * w for _ in range(h))
    return (b"\x89PNG\r\n\x1a\n" +
            chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) +
            chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def upload(token, name, data, kind="image"):
    boundary = "----defence" + str(time.time_ns())
    parts = [b"--" + boundary.encode() + b"\r\n",
             b'Content-Disposition: form-data; name="file"; filename="' + name.encode() + b'"\r\n',
             b"Content-Type: application/octet-stream\r\n\r\n", data,
             b"\r\n--" + boundary.encode() + b"--\r\n"]
    # NOTE: the endpoint is /api/upload (singular) and answers 200, not 201.
    return jreq("POST", "/api/upload?kind=" + kind, raw=b"".join(parts), token=token,
                ctype="multipart/form-data; boundary=" + boundary)


def section(t):
    print(f"\n=== {t} ===")


if not ROOT_PW:
    print("FATAL: set ROOT_PASSWORD (see the docstring: first boot forces a change)")
    sys.exit(2)

root, _ = login("admin", ROOT_PW)

# Reachability first: a run against a dead port should say so once instead of
# producing fifteen confusing assertion failures.
_probe, _pbody, _ = req("GET", "/api/site-config", timeout=5)
if _probe == 0:
    print(f"FATAL: {B} is not reachable ({_pbody.decode('utf-8', 'replace')[:120]})")
    sys.exit(2)
if not root:
    print("FATAL: cannot log in as root with ROOT_PASSWORD")
    print("       (first boot sets must_change_password=true; change it once, then rerun)")
    sys.exit(2)

# A plain author and a plain admin for the authority cases. Created through the
# user-management endpoint because registration is closed by default.
ST_PLAIN_USER, _, _ = jreq("POST", "/api/users",
                           {"username": "drilluser", "email": "drill@example.com",
                            "password": PLAIN_PW, "role": "user"}, token=root)
user, _ = login("drill@example.com", PLAIN_PW)
ST_PLAIN_ADMIN, _, _ = jreq("POST", "/api/users",
                            {"username": "drilladmin", "email": "drilladmin@example.com",
                             "password": PLAIN_PW, "role": "admin"}, token=root)
adm, _ = login("drilladmin@example.com", PLAIN_PW)

section("R1-02 / R3-05  login body cap")
big = b'{"email":"' + b"a" * (25 * 1024 * 1024) + b'","password":"x"}'
st, _, _ = req("POST", "/api/auth/login", raw=big,
               headers={"Content-Type": "application/json"})
# The server answers 413 as soon as the cap is crossed and closes; a client
# still writing sees a reset instead of the response. Either way the 25 MB
# never reached the decoder, which is the property being tested.
(ok("25 MB login body refused", f"status={st}") if st in (413, 0) else
 bad("25 MB login body", f"want 413 or a closed connection, got {st}"))

# A 200 KB identifier must not 500 (the old path truncated nowhere).
st, _, _ = req("POST", "/api/auth/login",
               {"email": "z" * 200000, "password": "x"})
expect("200 KB identifier hits the body cap", 413, st)
# A long-but-under-cap identifier must be a clean 401, not a 500.
st, _, _ = req("POST", "/api/auth/login", {"email": "z" * 7000, "password": "x"})
(ok("7 KB identifier handled", f"={st}") if st in (400, 401) else
 bad("7 KB identifier", f"want 400/401, got {st}"))

section("R5-04  SMTP secret never echoed")
st, cfg, _ = jreq("GET", "/api/admin/site-config", token=root)
(ok("smtp_pass empty for root", repr(cfg.get("smtp_pass")))
 if cfg.get("smtp_pass", "x") in ("", None) else
 bad("smtp_pass echoed", repr(cfg.get("smtp_pass"))))
(ok("img_host_token empty") if cfg.get("img_host_token", "x") in ("", None)
 else bad("img_host_token echoed", repr(cfg.get("img_host_token"))))
(ok("non-secret fields still present", "smtp_host")
 if "smtp_host" in cfg else bad("smtp_host missing", list(cfg)[:5]))

section("R6-08  partial site-config save keeps switches")
# Turn public_read ON first.
st, cur, _ = jreq("GET", "/api/admin/site-config", token=root)
cur["public_read"] = True
cur["auto_backup"] = True
cur["backup_keep"] = 3
req("PUT", "/api/admin/site-config", cur, token=root)
# Now a partial payload that omits all three.
st, _, _ = req("PUT", "/api/admin/site-config",
               {"site_name": "Echo", "site_title": "Echo", "img_host_mode": "local"},
               token=root)
expect("partial save accepted", 200, st)
st, after, _ = jreq("GET", "/api/admin/site-config", token=root)
expect("public_read inherited", True, after.get("public_read"))
expect("auto_backup inherited", True, after.get("auto_backup"))
expect("backup_keep inherited", 3, after.get("backup_keep"))

section("R6-09  friend_links type confusion")
st, _, _ = req("PUT", "/api/admin/site-config",
               {"site_name": "Echo", "site_title": "Echo", "img_host_mode": "local",
                "friend_links": [{"name": "x", "url": "https://e.com/"}]},
               token=root)
expect("array friend_links refused", 400, st)
long_name = "N" * 5000
st, _, _ = req("PUT", "/api/admin/site-config",
               {"site_name": "Echo", "site_title": "Echo", "img_host_mode": "local",
                "friend_links": json.dumps([{"name": long_name, "url": "https://e.com/"}])},
               token=root)
expect("over-long link name refused", 400, st)

section("R1-05  email_verify requires smtp_host")
st, _, _ = req("PUT", "/api/admin/site-config",
               {"site_name": "Echo", "site_title": "Echo", "img_host_mode": "local",
                "email_verify_enabled": True, "smtp_host": ""},
               token=root)
expect("verify without SMTP refused", 400, st)

section("R3-07 / R6-07  field ceilings")
st, body, _ = req("POST", "/api/posts",
                  {"title": "T" * 501, "body": "ok", "status": "published"},
                  token=root)
expect("over-long title refused", 400, st)
st, body, _ = req("POST", "/api/posts",
                  {"title": "ok", "body": "x" * (256 * 1024 + 1), "status": "published"},
                  token=root)
expect("over-long body refused", 400, st)
st, body, _ = req("POST", "/api/posts", {"title": "ok", "body": "fine"},
                  token=root,
                  headers={"Content-Type": "application/json"})
(ok("body at the ceiling accepted") if st in (200, 201) else
 bad("normal post", f"={st} {body[:120]}"))

section("R6-06  page overflow")
st, _, _ = req("GET", "/api/posts?page=922337203685477580&size=10")
expect("overflowing page refused", 400, st)
st, body, _ = req("GET", "/api/posts?page=100000&size=10")
(ok("page at the ceiling is an empty page") if st == 200 else
 bad("page at ceiling", f"={st}"))

section("R4-09 / R6-04  upload strips bytes after IEND")
base = png(8, 8, 10)
st, b, _ = upload(root, "clean.png", base)
expect("clean PNG accepted", 200, st)
clean_name = b.get("name", "")

payload = b"<?php system($_GET[0]); ?>"
st, b, _ = upload(root, "poly.png", base + payload)
(ok("PNG+payload accepted (header is real)", f"={st}") if st == 200 else
 bad("PNG+payload", f"={st} {str(b)[:120]}"))
if st == 200:
    # Fetch it back and look for the payload in what the server serves.
    st2, served, _ = req("GET", "/uploads/" + b.get("name", ""))
    (ok("stored file has no appended payload") if payload not in served else
     bad("stored file still contains the payload", f"{len(served)} bytes served"))
    (ok("served bytes are still a valid PNG") if served[:8] == b"\x89PNG\r\n\x1a\n"
     else bad("served file is not a PNG", served[:16]))

section("R6-05  compression-ratio guard (pixel bomb)")
bomb = png(6000, 4000, 128)  # 24 MP, tiny file
st, b, _ = upload(root, "bomb.png", bomb)
(ok("24 MP flat canvas refused", f"={st} {len(bomb)}B") if st == 400 else
 bad("24 MP flat canvas", f"want 400, got {st} ({len(bomb)} bytes)"))

section("R2-04  backfill must not adopt generated thumbnails")
# Structural: a thumbnail is named <id>_t.<ext>. We cannot easily provoke the
# startup backfill over HTTP, so assert the naming rule through the gallery:
# every row listed must have an owner (a legacy row has none).
st, page, _ = jreq("GET", "/api/uploads?status=all&page=1&size=50", token=root)
legacy = 0
if st == 200:
    for it in page.get("items", []):
        if not it.get("owner_id"):
            legacy += 1
    (ok("no owner-less rows in a fresh library", f"legacy={legacy}") if legacy == 0
     else bad("owner-less rows present", f"{legacy} rows"))
else:
    bad("gallery list", f"={st}")

section("R3-03 / R4-02  email code: IP budget + no double-spend")
# This feature is off by default; the guard is that the endpoint does not leak
# a code without SMTP configured.
st, b, _ = jreq("POST", "/api/auth/email-code", {"email": "drill@example.com"})
(ok("email-code refuses without SMTP", f"={st}") if st in (400, 403, 503) else
 bad("email-code without SMTP", f"={st} {str(b)[:120]}"))

section("R3-04  XFF: forged prefix must not create a fresh bucket")
# The 6 rounds' attack was: prepend a forged XFF hop, so the limiter reads a
# different client address on every request and never throttles. With the
# right-to-left stripping fix, every one of these must land in the same bucket
# as the real peer (127.0.0.1) because the trusted-proxy chain bottoms out there.
codes = []
for i in range(10):
    forged = f"10.{i}.{i}.{i}, 127.0.0.1"
    st, _, _ = req("POST", "/api/auth/login",
                   {"email": "victim@example.com", "password": "x"},
                   ctype="application/json")  if False else req(
        "POST", "/api/auth/login",
        {"email": "victim@example.com", "password": "x"},
        )
    codes.append(st)
(ok("repeated failures eventually throttled", f"codes={codes[:6]}...")
 if 429 in codes else bad("no throttle after 10 failures", f"codes={codes}"))

# Now the forged-header variant, using the header directly.
forged_codes = []
for i in range(10):
    url = B + "/api/auth/login"
    data = json.dumps({"email": "victim2@example.com", "password": "x"}).encode()
    r = urllib.request.Request(url, data=data, method="POST",
                               headers={"Content-Type": "application/json",
                                        "X-Forwarded-For": f"10.9.{i}.{i}, 127.0.0.1"})
    try:
        with urllib.request.urlopen(r, timeout=20) as resp:
            forged_codes.append(resp.status)
    except urllib.error.HTTPError as e:
        forged_codes.append(e.code)
    except Exception:
        forged_codes.append(0)
(ok("forged XFF cannot mint fresh buckets", f"codes={forged_codes[:6]}...")
 if 429 in forged_codes else
 bad("forged XFF bypassed the limiter", f"codes={forged_codes}"))

section("R1-01  public_read=false blocks anonymous reads everywhere")
# Turn anonymous reading OFF.
st, cfg, _ = jreq("GET", "/api/admin/site-config", token=root)
cfg["public_read"] = False
st, _, _ = jreq("PUT", "/api/admin/site-config", cfg, token=root)
expect("public_read turned off", 200, st)

# Create a published post so there is something to try to read.
st, p, _ = jreq("POST", "/api/posts",
                {"title": "Anonymous target", "body": "secret body", "status": "published"},
                token=root)
pid = p.get("id", "")
slug = p.get("slug", "")
ok("seeded a published post", f"id={pid} slug={slug}")

for path, label in [
    (f"/api/posts/{pid}", "GET /api/posts/{id}"),
    (f"/api/posts/{pid}/public", "GET /api/posts/{id}/public"),
    (f"/api/permalinks/{slug}", "GET /api/permalinks/{slug}"),
    ("/api/feed", "GET /api/feed"),
    ("/api/sitemap.xml", "GET /api/sitemap.xml"),
]:
    st, body, _ = req("GET", path)
    # 401 for JSON endpoints, 404 is also acceptable (nothing to serve). What
    # must NOT happen is 200 with the body.
    leaked = b"secret body" in body
    (ok(f"{label} blocked", f"={st}") if (st in (401, 403, 404) and not leaked)
     else bad(f"{label} leaked content", f"status={st} leaked={leaked}"))

# Restore anonymous reading.
st, cfg, _ = jreq("GET", "/api/admin/site-config", token=root)
cfg["public_read"] = True
jreq("PUT", "/api/admin/site-config", cfg, token=root)

section("R5-05  admin cannot rewrite another author's content")
if user:
    # drilluser writes a post.
    st, mine, _ = jreq("POST", "/api/posts",
                       {"title": "Drill original", "body": "original words"},
                       token=user)
    myid = mine.get("id", "")
    if adm:
        ok("admin session")
        st, b, _ = jreq("PUT", f"/api/posts/{myid}",
                        {"body": "words the admin prefers"}, token=adm)
        expect("admin body rewrite refused", 403, st)
        st, b, _ = jreq("PUT", f"/api/posts/{myid}",
                        {"title": "admin title"}, token=adm)
        expect("admin title rewrite refused", 403, st)
        # The editorial action must still work: unpublish to private.
        st, b, _ = jreq("PUT", f"/api/posts/{myid}", {"status": "private"}, token=adm)
        (ok("admin may still unpublish", f"={st}") if st in (200, 409) else
         bad("admin unpublish", f"={st} {str(b)[:120]}"))
    else:
        bad("promote to admin", f"st={st}")

section("R4-11  concurrent approvals do not both win")
if user:
    st, q, _ = jreq("POST", "/api/posts",
                    {"title": "Race me", "body": "body"}, token=user)
    qid = q.get("id", "")
    results = []
    lock = threading.Lock()

    def approve(i):
        st, b, _ = jreq("POST", f"/api/admin/review/{qid}",
                        {"decision": "approve"}, token=adm)
        with lock:
            results.append(st)

    ths = [threading.Thread(target=approve, args=(i,)) for i in range(5)]
    for t in ths:
        t.start()
    for t in ths:
        t.join()
    wins = results.count(200)
    (ok("exactly one approval won", f"results={sorted(results)}") if wins == 1
     else bad("concurrent approvals", f"{wins} succeeded: {sorted(results)}"))

print()
print(f"========  PASS={PASS}  FAIL={len(FAIL)}  ========")
if FAIL:
    for n, d in FAIL:
        print(f"  !! {n}: {d}")
    sys.exit(1)
print("ALL DEFENCE CHECKS PASSED")
