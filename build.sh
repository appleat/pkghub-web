#!/usr/bin/env bash
# 一键构建脚本（Linux/macOS）：同时产出 windows/linux/darwin 三个平台的可执行文件。
# 用法：./build.sh [版本号，默认 1.0.0]
set -euo pipefail
VERSION=${1:-1.0.0}
LDFLAGS="-s -w -X main.version=${VERSION}"
mkdir -p dist
for GOOS in windows linux darwin; do
  EXT=""
  if [ "$GOOS" = "windows" ]; then EXT=".exe"; fi
  echo "==> 构建 ${GOOS}/amd64"
  GOOS="$GOOS" GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "dist/pkghub-${GOOS}-amd64${EXT}" .
done
echo "构建完成，输出目录：dist/"
