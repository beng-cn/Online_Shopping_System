#!/bin/bash
# 安全提交脚本 — 本项目适配版
# 用法: bash git-safe-push.sh "提交信息"
set -e

MSG="${1:-update}"
BACKUP="敏感信息.txt"

echo "=== 1. 扫描敏感信息 ==="
> "$BACKUP"
# 本项目敏感配置均在 backend/configs/dev.yaml（已加入.gitignore）
# 扫描其他可能泄露密码的文件
for f in backend/cmd/server/main.go; do
  [ -f "$f" ] || continue
  grep -n '181871ZX\|password\|admin123' "$f" 2>/dev/null | while read line; do
    echo "${f}:${line}" >> "$BACKUP"
  done
done
echo "  敏感信息行数: $(wc -l < "$BACKUP")"

echo "=== 2. 验证敏感文件已排除 ==="
git check-ignore backend/configs/dev.yaml && echo "  ✅ dev.yaml 已被 .gitignore 排除" || echo "  ⚠️ dev.yaml 未被排除！"
git check-ignore backend/账密.txt && echo "  ✅ 账密.txt 已被 .gitignore 排除" || echo "  ⚠️ 账密.txt 未被排除！"

echo "=== 3. 提交推送 ==="
git add -A
git commit -m "$MSG"
git push || echo "⚠️ 推送失败，请稍后手动 git push"

echo "=== 4. 编译验证 ==="
go build ./... && echo "✅ 后端编译通过" || { echo "❌ 后端编译失败"; exit 1; }

echo "=== 5. 前端构建验证 ==="
cd frontend && npm run build 2>&1 | grep -q "✓ built" && echo "✅ 前端构建通过" || { echo "❌ 前端构建失败"; }
cd ..

echo "✅ 提交完成"
