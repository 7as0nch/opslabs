#!/usr/bin/env bash
# ------------------------------------------------------------------
# opslabs / scripts/fetch-v86.sh
# ------------------------------------------------------------------
# 把 v86 + BusyBox linux 镜像所需的 5 个二进制资源下载到后端 bundle 目录,
# 后端 embed.FS 会把它们打进二进制,由 bundle handler 下发给前端,
# 完全避开 copy.sh CDN 被墙的问题。
#
# 用法:
#   ./scripts/fetch-v86.sh                     # 从默认镜像拉(copy.sh 官方)
#   V86_MIRROR=https://xxx/v86 ./fetch-v86.sh  # 自定义镜像
#
# 执行成功后会生成:
#   backend/internal/scenario/bundles/wasm-linux-hello/vendor/
#     ├── libv86.js        (~800KB)
#     ├── v86.wasm         (~500KB)
#     ├── seabios.bin      (~96KB)
#     ├── vgabios.bin      (~40KB)
#     └── linux.iso        (~4MB)
#
# 之后跑一次 `cd backend && go build ./...` 让 embed.FS 生效。
#
# 文件来源:
#   copy.sh/v86 的 GitHub release 资源(GPLv2 开源);
#   镜像站:如果 copy.sh 在你所在网络不可达,可以把 V86_MIRROR 改成
#   反代回源或者自己 fork 的 release。jsdelivr 只能代理 npm,linux.iso
#   不在 npm,所以 jsdelivr 无法完全代替 copy.sh。
# ------------------------------------------------------------------

set -euo pipefail

V86_MIRROR="${V86_MIRROR:-https://copy.sh/v86}"

# 脚本从任意位置调用都能找到正确目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VENDOR_DIR="$REPO_ROOT/backend/internal/scenario/bundles/wasm-linux-hello/vendor"

mkdir -p "$VENDOR_DIR"

echo "[fetch-v86] mirror = $V86_MIRROR"
echo "[fetch-v86] target = $VENDOR_DIR"
echo ""

# 路径:<mirror 下相对路径>。落地时保留目录结构(build/ bios/ images/),
# 这样前端 index.html 里不论走 ./vendor 还是 https://copy.sh/v86,
# 后缀 URL 完全一致,bootLib/bootEmulator 不需要做路径切换。
FILES=(
  "build/libv86.js"
  "build/v86.wasm"
  "bios/seabios.bin"
  "bios/vgabios.bin"
  "images/linux.iso"
)

for rel in "${FILES[@]}"; do
  url="$V86_MIRROR/$rel"
  out="$VENDOR_DIR/$rel"
  mkdir -p "$(dirname "$out")"
  echo "[fetch-v86] GET  $url"
  # curl:
  #   -f  非 200 失败退出
  #   -L  跟随 302
  #   -sS 静默 + 显示错误
  #   -o  输出文件
  curl -fLsS -o "$out" "$url"
  size=$(wc -c <"$out" | tr -d ' ')
  echo "[fetch-v86] ok   $rel ($size bytes)"
done

echo ""
echo "[fetch-v86] 完成。接下来:"
echo "  cd backend && go build ./..."
echo "  # 让 //go:embed all:wasm-linux-hello 把 vendor/ 一起打进二进制"
