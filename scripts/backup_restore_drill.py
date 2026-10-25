#!/usr/bin/env python3
"""Backup / restore drill: can the site actually be brought back?

Why this is a separate script
-----------------------------
`internal/services/backup_test.go` proves `ExportBackup` and `ImportBackup`
work as functions. That is not the question an operator asks. The question is
whether the file produced tonight can be put back onto a machine that has
nothing but the binary and an empty directory -- and whether the site then
behaves like itself: same passwords, same articles, same image references, no
session that was issued before the restore still working.

Everything in that sentence is a property of the ROUND TRIP across two
instances, and it is the one class of failure unit tests structurally cannot
see. The first run of this drill found exactly such a defect: a route-level
request-body ceiling (added to close a memory-exhaustion hole) defaulted to
1 MiB and silently capped `/api/backup/import` at 1 MiB, while its handler
promised 20 MiB. A 1.25 MiB dump exported fine and was refused on import. The
site had backups, and none of them could be restored.

Usage
-----
Three phases, because a restore needs two servers and this process cannot start
them for you. Run them in order; each one exits non-zero on the first failure.

    # 1. an empty instance (the "new box"), plus a source instance to back up
    #    -- see the header of scripts/defence_smoke.py for the env shape; the
    #    only requirements here are two ports and two data directories.
    XDG_TMP=/tmp/backup-drill
    mkdir -p $XDG_TMP/a/{uploads,backups} $XDG_TMP/b/{uploads,backups}

    # source site on :18097
    PORT=18097 DB_PATH=$XDG_TMP/a/a.db UPLOAD_DIR=$XDG_TMP/a/uploads \\
      BACKUP_DIR=$XDG_TMP/a/backups JWT_SECRET=<32+ chars> APP_ENV=production \\
      CORS_ORIGINS=https://drill.local ADMIN_INITIAL_PASSWORD='DrillPass1!' \\
      ./bin/lopezpress

    # target site on :18096 (empty database, uploads copied over separately)
    PORT=18096 DB_PATH=$XDG_TMP/b/b.db UPLOAD_DIR=$XDG_TMP/b/uploads \\
      BACKUP_DIR=$XDG_TMP/b/backups JWT_SECRET=<32+ chars, DIFFERENT> \\
      APP_ENV=production CORS_ORIGINS=https://drill.local \\
      ADMIN_INITIAL_PASSWORD='DrillPass1!' ./bin/lopezpress

    # 2. the phases
    python scripts/backup_restore_drill.py populate --state $XDG_TMP/state.json
    python scripts/backup_restore_drill.py restore  --state $XDG_TMP/state.json
    python scripts/backup_restore_drill.py ceiling  --state $XDG_TMP/state.json

`populate` runs against the source (default :18097), the other two against the
target (default :18096). Override with SOURCE_URL / TARGET_URL.

What each phase proves
----------------------
populate  a site can be filled and backed up; the dump exceeds the 1 MiB default
          route ceiling on purpose, because that is the case that broke
restore   the dump goes back in, the users table really was replaced, the
          pre-restore sessions are dead, the articles and their image
          references survive, and the instance reports ready
ceiling   the same bytes are refused by a 1 MiB route and accepted by the
          backup route -- i.e. the import's size ceiling is a property of the
          route declaration, not a coincidence
"""
import json
import os
import struct
import sys
import time
import urllib.error
import urllib.request
import zlib

SOURCE = os.environ.get("SOURCE_URL", "http://127.0.0.1:18097").rstrip("/")
TARGET = os.environ.get("TARGET_URL", "http://127.0.0.1:18096").rstrip("/")
BOOTSTRAP = os.environ.get("BOOTSTRAP_PASSWORD", "DrillPass1!")
SOURCE_PW = os.environ.get("SOURCE_PASSWORD", "DrillPass2!")
TARGET_PW = os.environ.get("TARGET_PASSWORD", "DrillPass3!")
CONFIRM = "REPLACE-ALL-DATA"

PASSED = 0


def ok(msg):
    global PASSED
    PASSED += 1
    print(f"  PASS  {msg}")


def bad(msg):
    print(f"  FAIL  {msg}")
    sys.exit(1)


def note(msg):
    print(f"  ....  {msg}")


def req(base, method, path, body=None, token=None, raw=None, ctype=None,
        timeout=180):
    """One request. Returns (status, bytes); status 0 means the connection died.

    Status 0 is not an error to swallow: the server answers 413 as soon as
    Content-Length crosses a ceiling and closes, so a client still uploading
    sees the socket die instead of the response. Both outcomes mean the body
    never reached a decoder.
    """
    data = raw
    headers = {}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if ctype:
        headers["Content-Type"] = ctype
    if token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(base + path, data=data, headers=headers,
                               method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:  # noqa: BLE001
        return 0, str(e).encode()


def jreq(base, *a, **kw):
    status, body = req(base, *a, **kw)
    try:
        return status, json.loads(body)
    except Exception:  # noqa: BLE001
        return status, {}


def login(base, ident, password):
    status, body = jreq(base, "POST", "/api/auth/login",
                        {"email": ident, "password": password})
    if status != 200:
        return None, status, body
    return body["token"], status, body


def settle_bootstrap(base, password):
    """A fresh install refuses everything until the bootstrap password changes."""
    token, status, body = login(base, "admin", password)
    if status != 200:
        bad(f"{base}: cannot log in with the bootstrap password ({status} {body})")
    if body["user"].get("must_change_password"):
        status, res = jreq(base, "POST", "/api/auth/change-password",
                           {"current_password": password, "new_password": TARGET_PW},
                           token=token)
        if status != 200:
            bad(f"{base}: cannot clear the must-change-password gate ({status} {res})")
        note("cleared the must-change-password gate")
        token, status, _ = login(base, "admin", TARGET_PW)
        if status != 200:
            bad(f"{base}: cannot log in after changing the bootstrap password")
    return token


def png(w, h, shade):
    def chunk(tag, data):
        return (struct.pack(">I", len(data)) + tag + data +
                struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF))
    raw = b"".join(b"\x00" + bytes([shade, shade, shade]) * w for _ in range(h))
    return (b"\x89PNG\r\n\x1a\n" +
            chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) +
            chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def upload(base, token, name, data):
    boundary = "----drill" + str(time.time_ns())
    parts = [b"--" + boundary.encode() + b"\r\n",
             b'Content-Disposition: form-data; name="file"; filename="' +
             name.encode() + b'"\r\n',
             b"Content-Type: application/octet-stream\r\n\r\n", data,
             b"\r\n--" + boundary.encode() + b"--\r\n"]
    return jreq(base, "POST", "/api/upload?kind=image", raw=b"".join(parts),
                token=token,
                ctype="multipart/form-data; boundary=" + boundary)


# --- phase 1: fill the source site and export ------------------------------

def phase_populate(state_path):
    print("=== populate: fill the source site and export ===")
    token = settle_bootstrap(SOURCE, BOOTSTRAP)
    ok("source site is up and has a usable root session")

    # A different shade each run: identical bytes would be deduplicated onto the
    # existing upload row, whose ref_count includes articles from earlier runs.
    status, res = upload(SOURCE, token, "drill-cover.png",
                         png(40, 30, (int(time.time()) % 200) + 20))
    if status != 200:
        bad(f"upload: {status} {res}")
    img_name = res.get("name") or res.get("file", {}).get("name")
    img_url = res.get("url") or ("/uploads/" + str(img_name))
    if not img_name:
        bad(f"upload response carries no file name: {res}")
    ok(f"uploaded {img_name}")

    status, post = jreq(SOURCE, "POST", "/api/posts", {
        "title": "Drill post survives a restore",
        "body": f"Before the restore this body references ![]({img_url}) and nothing else.",
        "status": "published",
    }, token=token)
    if status not in (200, 201):
        bad(f"create post: {status} {post}")
    post_id = post["id"]
    ok(f"created post {post_id}")

    status, post2 = jreq(SOURCE, "POST", "/api/posts", {
        "title": "Second post, text only", "body": "no images here",
        "status": "published",
    }, token=token)
    if status not in (200, 201):
        bad(f"second post: {status} {post2}")
    ok("created a second, text-only post")

    # Bulk content, so the dump is larger than the 1 MiB default route ceiling.
    # A real site crosses that line with a few dozen ordinary articles, and that
    # is the case the route table used to break -- see phase `ceiling`.
    bulk = "lorem ipsum dolor sit amet " * 6000  # ~162 KB, under the 256 KB cap
    bulk_posts = 8
    for i in range(bulk_posts):
        status, res = jreq(SOURCE, "POST", "/api/posts", {
            "title": f"Bulk article {i}", "body": bulk, "status": "published",
        }, token=token)
        if status not in (200, 201):
            bad(f"bulk post {i}: {status} {res}")
    ok(f"added {bulk_posts} bulk articles so the dump exceeds 1 MiB")

    status, lib = jreq(SOURCE, "GET", "/api/uploads", token=token)
    items = lib.get("items") or lib.get("files") or []
    counts = [f.get("ref_count") for f in items if f.get("name") == img_name]
    if not counts:
        bad(f"the uploaded image is not in the library: {json.dumps(lib)[:200]}")
    if counts[0] < 1:
        bad(f"ref_count for {img_name} is {counts[0]} before the backup")
    note(f"the ledger counts ref_count={counts[0]} before the backup")
    ok("the image ledger sees the reference before the backup")

    status, meta = jreq(SOURCE, "POST", "/api/backup", token=token)
    if status != 200:
        bad(f"export: {status} {meta}")
    note(f"export wrote {meta.get('filename')} ({meta.get('size')} bytes)")

    # The payload is a downloadable file, not the response body: the operator's
    # round trip is export -> download -> import.
    status, raw = req(SOURCE, "GET",
                      "/api/backup/download?file=" + meta["filename"],
                      token=token)
    if status != 200:
        bad(f"download: {status} {raw[:200]}")
    backup = json.loads(raw)
    if not any(p.get("id") == post_id for p in backup.get("posts") or []):
        bad("the exported dump does not contain the post that was just created")
    if len(raw) <= (1 << 20):
        bad(f"the dump is only {len(raw)} bytes; it must exceed the 1 MiB default "
            f"ceiling for phase `ceiling` to mean anything")
    ok(f"the dump is {len(raw)} bytes ({len(raw) / 1048576:.2f} MiB), above 1 MiB")

    with open(state_path, "w", encoding="utf-8") as f:
        json.dump({"backup": backup, "post_id": post_id, "img": img_name,
                   "source_token": token}, f)
    note(f"wrote {state_path}")
    print(f"\npopulate: {PASSED} checks passed")
    print("Next: copy the SOURCE uploads directory into the TARGET's, then run")
    print("      python scripts/backup_restore_drill.py restore --state <file>")


# --- phase 2: restore into the target --------------------------------------

def phase_restore(state_path):
    print("=== restore: a fresh instance takes the dump ===")
    with open(state_path, encoding="utf-8") as f:
        state = json.load(f)
    payload = state["backup"]
    raw_payload = json.dumps(payload).encode()
    note(f"payload is {len(raw_payload)} bytes")

    token = settle_bootstrap(TARGET, BOOTSTRAP)
    ok("target instance is up and has its own root")

    # Validation before destruction. An incompatible dump must be refused with a
    # code that says so, not half-applied.
    status, res = jreq(TARGET, "POST", "/api/backup/import",
                       {"schema_version": 9999, "app_version": "x", "confirm": CONFIRM},
                       token=token)
    if status != 400:
        bad(f"an incompatible schema_version must be 400, got {status} {res}")
    ok("an incompatible dump is refused before any write")

    # The confirmation phrase exists so a mis-click cannot wipe a site.
    status, res = jreq(TARGET, "POST", "/api/backup/import",
                       dict(payload, confirm=""), token=token)
    if status not in (400, 428):
        bad(f"an import without the confirm phrase must be refused, got {status} {res}")
    ok("the confirm phrase is required")

    status, res = jreq(TARGET, "POST", "/api/backup/import?dry_run=1",
                       dict(payload, confirm=CONFIRM), token=token)
    if status != 200:
        bad(f"dry run: {status} {res}")
    ok("dry run accepts the payload")

    status, res = jreq(TARGET, "POST", "/api/backup/import",
                       dict(payload, confirm=CONFIRM), token=token)
    if status != 200:
        bad(f"import: {status} {res}")
    note(f"stats: {json.dumps(res.get('stats'))}, snapshot: {res.get('snapshot_file')}")
    ok(f"restored {len(raw_payload)} bytes in one request")

    # The users table was replaced, so the SOURCE password is the live one.
    token, status, res = login(TARGET, "admin", SOURCE_PW)
    if status != 200:
        bad(f"after the restore the source password must work, got {status} {res}")
    ok("the restored account logs in with its original password")

    for stale in (BOOTSTRAP, TARGET_PW):
        _, status, _ = login(TARGET, "admin", stale)
        if status == 200:
            bad(f"the pre-restore password {stale!r} still works — the import did "
                f"not actually replace the users table")
    ok("neither pre-restore password works any more")

    # A session issued before the restore must be dead: it was issued against
    # users that no longer exist.
    status, _ = req(TARGET, "GET", "/api/auth/me", token=token)
    if status != 200:
        bad(f"the post-restore session is broken ({status})")
    status, _ = req(TARGET, "GET", "/api/auth/me", token=state["source_token"])
    if status != 401:
        bad(f"a session from the source site is still valid on the target "
            f"({status}); the token epoch was not raised")
    ok("sessions from before the restore are dead")

    status, post = jreq(TARGET, "GET", "/api/posts/" + state["post_id"])
    if status != 200:
        bad(f"the restored post is not readable: {status} {post}")
    if post.get("title") != "Drill post survives a restore":
        bad(f"title changed in transit: {post.get('title')!r}")
    if state["img"] not in post.get("body", ""):
        bad(f"the body lost its image reference: {post.get('body', '')[:200]!r}")
    ok("the restored post keeps its title and body")

    status, listing = jreq(TARGET, "GET", "/api/posts?page=1&size=100")
    count = len(listing.get("items") or listing.get("posts") or [])
    total = listing.get("total")
    if count < 19:
        bad(f"only {count} posts visible after restoring a {len(payload['posts'])}-post dump")
    ok(f"the restored site lists {count} articles (total={total})")

    # The ledger must be rebuilt from the restored posts. Left stale, the
    # gallery's "deleting this image breaks N articles" warning becomes a lie.
    status, lib = jreq(TARGET, "GET", "/api/uploads", token=token)
    items = lib.get("items") or lib.get("files") or []
    counts = [f.get("ref_count") for f in items if f.get("name") == state["img"]]
    if not counts:
        bad(f"the restored image is missing from the library: {json.dumps(lib)[:200]}")
    if counts[0] != 1:
        bad(f"ref_count for {state['img']} is {counts[0]} after the restore, "
            f"expected 1 — the ledger was not rebuilt")
    ok("the image ledger was rebuilt from the restored content")

    status, _ = req(TARGET, "GET", "/health/ready")
    if status != 200:
        bad(f"the restored instance is not ready ({status})")
    ok("the restored instance reports ready")

    print(f"\nrestore: {PASSED} checks passed")


# --- phase 3: prove the ceiling is what changed ----------------------------

def phase_ceiling(state_path):
    print("=== ceiling: the same bytes, two routes, two verdicts ===")
    with open(state_path, encoding="utf-8") as f:
        state = json.load(f)

    token, status, res = login(TARGET, "admin", SOURCE_PW)
    if status != 200:
        bad(f"cannot log in on the restored target ({status} {res})")
    note("logged in with the restored account")

    raw = json.dumps(state["backup"]).encode()
    note(f"payload is {len(raw)} bytes ({len(raw) / 1048576:.2f} MiB)")

    # A route still on the 1 MiB default. The body is valid JSON; the only
    # reason to refuse it is size.
    status, _ = req(TARGET, "POST", "/api/posts", raw=raw, token=token)
    if status not in (413, 0):
        bad(f"a route on the 1 MiB default accepted {len(raw)} bytes ({status}) — "
            f"the default ceiling is not being applied")
    ok("a route on the 1 MiB default refuses the same body")

    status, _ = req(TARGET, "POST", "/api/backup/import?dry_run=1",
                    body=dict(state["backup"], confirm=CONFIRM), token=token)
    if status != 200:
        bad(f"the backup route refused it ({status})")
    ok("the backup route accepts it")

    print("\n  Same bytes, same Content-Length; only the route's declared ceiling")
    print("  differs. With maxBody left at the default, /api/backup/import returns")
    print("  413 for any dump over 1 MiB: an exported backup that cannot be restored.")
    print(f"\nceiling: {PASSED} checks passed")


def main():
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print(__doc__)
        return 0
    phase = sys.argv[1]
    state_path = None
    if "--state" in sys.argv:
        state_path = sys.argv[sys.argv.index("--state") + 1]
    if not state_path:
        print("FATAL: --state <file> is required")
        return 2
    if phase == "populate":
        phase_populate(state_path)
    elif phase == "restore":
        phase_restore(state_path)
    elif phase == "ceiling":
        phase_ceiling(state_path)
    else:
        print(f"FATAL: unknown phase {phase!r} (populate|restore|ceiling)")
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
