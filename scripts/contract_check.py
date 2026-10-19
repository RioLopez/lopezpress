#!/usr/bin/env python
# Cross-stack contract checker: every request<T>() path in web/src/core/api.ts
# must exist on the backend router with the same HTTP method, and every
# backend route is listed so gaps (e.g. an endpoint with no frontend caller)
# are visible. Run from the repo root:  python scripts/contract_check.py
#
# Why this exists: the round-33 as-built audit found a silent contract drift
# (CreatePostInput lacked `cover` while the editor already sent it — Go's JSON
# decoder drops unknown fields, so the cover vanished at creation time).
# Path/method drift is invisible to tsc and go build; only an explicit
# cross-check catches it. Extend it with body-field diffing if drift recurs.
import re, io

src = io.open('internal/router/router.go', encoding='utf-8').read()
routes = set()
for m in re.finditer(r'mux\.Handle(?:Func)?\("(GET|POST|PUT|DELETE) ([^"]+)"', src):
    routes.add((m.group(1), m.group(2)))

fe = io.open('web/src/core/api.ts', encoding='utf-8').read()
fe_calls = []
# request<T>('path' | `path`, ... { method: 'X' ... })  —— 兼容单引号与反引号
for m in re.finditer(r"request<[^>]*>\(\s*[`']([^`']+)[`'](?:,\s*\{[^}]*?method:\s*'(\w+)')?", fe):
    path, method = m.group(1), (m.group(2) or 'GET')
    p = re.sub(r'\$\{[^}]*\}', '{id}', path).split('?')[0]
    fe_calls.append((method, p))
# raw fetch('path' / fetch(`path`)  —— downloadBackup / uploadImage
for m in re.finditer(r"fetch\(\s*[`']([^`']+)[`']", fe):
    p = re.sub(r'\$\{[^}]*\}', '{id}', m.group(1)).split('?')[0]
    if not p.startswith('http'):  # skip absolute SM.MS urls
        fe_calls.append(('GET', p))  # method refined below for known POSTs
# uploadImage posts FormData to /api/upload
if "'/api/upload'" in fe or '`/api/upload' in fe:
    fe_calls = [('POST', p) if p == '/api/upload' else (mth, p) for mth, p in fe_calls]

def norm(p):
    return re.sub(r'\{[^}]+\}', '{id}', p)

be = {(m, norm(p)) for m, p in routes}
feset = set(fe_calls)

print('=== 方法不匹配(前端 method+path 在后端不存在) ===')
bad = [(m, p) for m, p in sorted(feset) if (m, p) not in be]
for m, p in bad:
    print(f' - {m} {p}')
if not bad:
    print(' (无)')

print()
print('=== 后端路由前端从未调用 ===')
used = {norm(p) for _, p in feset}
unused = [(m, p) for m, p in sorted(routes) if norm(p) not in used]
for m, p in unused:
    print(f' - {m} {p}')
if not unused:
    print(' (无)')

print()
print(f'前端调用 {len(feset)} 个(方法+路径),后端路由 {len(routes)} 个')
