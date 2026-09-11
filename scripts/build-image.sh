#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -ne 1 || ! $1 =~ ^v?[0-9A-Za-z][0-9A-Za-z._-]*$ ]]; then
  printf '用法: %s <版本，例如 v1.2.3>\n' "$0" >&2
  exit 64
fi

version=$1
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    printf '不支持的宿主机架构: %s\n' "$(uname -m)" >&2
    exit 65
    ;;
esac

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
image="sk5proxy:${version}"
archive="${repo_root}/dist/sk5proxy-${version}-linux-${arch}.tar.gz"
checksum="${archive}.sha256"

if [[ -e $archive || -e $checksum ]]; then
  printf '拒绝覆盖已有产物: %s（请使用新版本或先显式移走旧文件）\n' "$archive" >&2
  exit 73
fi

mkdir -p -- "${repo_root}/dist"
if docker buildx version >/dev/null 2>&1; then
  docker buildx build --load --platform "linux/${arch}" -t "$image" "$repo_root"
else
  printf '未检测到 buildx，使用本机架构的 Docker 传统构建器。\n' >&2
  docker build -t "$image" "$repo_root"
fi

docker tag "$image" sk5proxy:offline
temporary="${archive}.tmp"
trap 'rm -f -- "$temporary"' EXIT
docker save "$image" sk5proxy:offline | gzip -n > "$temporary"
mv -- "$temporary" "$archive"
(
  cd -- "${repo_root}/dist"
  sha256sum "$(basename -- "$archive")" > "$(basename -- "$checksum")"
)
trap - EXIT

printf '镜像: %s（同时标记 sk5proxy:offline）\n产物: %s\n校验: %s\n' "$image" "$archive" "$checksum"
