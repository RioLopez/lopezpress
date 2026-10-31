#!/usr/bin/env bash
# 交叉编译 Linux amd64 静态二进制，并把版本号烙进产物。
#
# 版本号的单一真相源是仓库根目录的 VERSION 文件；commit / 构建时间 / 工作树
# 是否干净取自 git。四个值都通过 -ldflags -X 注入 internal/version，所以部署
# 之后有两个地方能回答"这台机器跑的是哪个构建"：
#
#   /opt/lopezpress/lopezpress -version
#   curl -s http://127.0.0.1:8081/health
#
# 产物同时写两份：带版本号的文件名（可追溯、可并存）和一个固定别名
# bin/lopezpress（部署脚本和文档都引用它，改版本不用改脚本）。
set -euo pipefail

cd "$(dirname "$0")/.."

# Go toolchain floor.
#
# go.mod declares `go 1.26.10` because every Go release below it ships the
# standard library with reachable advisories in net/http, net/url, crypto/x509
# and encoding/asn1 — 46 of them, measured with govulncheck on Go 1.22.12.
# Building with an older toolchain silently reintroduces all of them into the
# binary, so the check below is not a formality.
#
# Go 1.21+ downloads the required toolchain on demand, which is why this usually
# just works. The exception is a machine with GOSUMDB=off (this one): the
# download is refused for lack of checksum verification, and the error reads
# "checksum database disabled" rather than anything about versions.
need_go="1.26.10"
have_go="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
echo "go    : ${have_go:-unknown} (need >= ${need_go})"

VERSION="$(tr -d '[:space:]' < VERSION 2>/dev/null || true)"
VERSION="${VERSION:-0.0.0-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# 工作树脏的时候仍然允许构建（本地验证需要），但把 dirty=true 烙进去：一个
# 无法从某个 tag 复现的二进制，应该自己说出来。
if git diff --quiet 2>/dev/null && git diff --cached --quiet 2>/dev/null; then
	DIRTY="false"
else
	DIRTY="true"
fi

LDFLAGS="-s -w \
  -X lopezpress/internal/version.Version=${VERSION} \
  -X lopezpress/internal/version.Commit=${COMMIT} \
  -X lopezpress/internal/version.BuildTime=${BUILT} \
  -X lopezpress/internal/version.Dirty=${DIRTY}"

mkdir -p bin
OUT="bin/lopezpress-${VERSION}-linux-amd64"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-trimpath -ldflags="$LDFLAGS" \
	-o "$OUT" ./cmd/server || {
	cat >&2 <<'EOF'

build failed. If the message mentions "checksum database disabled", this shell
has GOSUMDB=off, which blocks the toolchain download that `go 1.26.10` in
go.mod triggers. Re-run with:

    GOSUMDB=sum.golang.org bash deploy/build.sh

EOF
	exit 1
}

cp -f "$OUT" bin/lopezpress

echo "built : $(pwd)/${OUT}"
echo "alias : bin/lopezpress"
echo "size  : $(du -h "$OUT" | cut -f1)"
echo "stamp : version=${VERSION} commit=${COMMIT} dirty=${DIRTY} built=${BUILT}"
