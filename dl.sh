#!/bin/bash

# ==================== 默认配置 ====================
DEFAULT_PROXY="http://localhost:8080"
DEFAULT_PASS="20260625"
# ====================================================

if [ -z "$1" ]; then
    echo "使用方法: $0 <下载链接> [密码] [服务器地址]"
    echo "示例: $0 \"https://example.com\""
    exit 1
fi

TARGET_URL="$1"
PASSWORD="${2:-$DEFAULT_PASS}"
PROXY_HOST="${3:-$DEFAULT_PROXY}"
PROXY_HOST="${PROXY_HOST%/}"

# 内部函数：对目标链接进行安全的 URL 编码，防止 & 和 ? 导致终端解析截断
urlencode() {
    local string="${1}"
    local strlen=${#string}
    local encoded=""
    local pos c o

    for (( pos=0 ; pos<strlen ; pos++ )); do
        c=${string:$pos:1}
        case "$c" in
            [-_.~a-zA-Z0-9] ) encoded="${encoded}${c}" ;;
            * ) o=$(printf '%%%02X' "'$c")
                encoded="${encoded}${o}" ;;
        esac
    done
    echo "${encoded}"
}

ENCODED_URL=$(urlencode "$TARGET_URL")

echo "========================================="
echo "📥 正在准备通过中转服务下载..."
echo "🔗 原始目标: $TARGET_URL"
echo "========================================="

# 智能嗅探并选择系统中最优的下载工具（已调整为：优先使用 wget，其次 curl，最后 aria2c）
if command -v wget &> /dev/null; then
    echo "📦 检测到 wget，正在启动标准中转下载..."
    wget --post-data="password=${PASSWORD}&target=${ENCODED_URL}" \
         --content-disposition \
         "${PROXY_HOST}/api/get-ticket"

elif command -v curl &> /dev/null; then
    echo "🗂️ 检测到 curl，正在启动流式安全下载..."
    curl -L \
         -d "password=${PASSWORD}" \
         --data-urlencode "target=${TARGET_URL}" \
         --remote-header-name \
         -O \
         "${PROXY_HOST}/api/get-ticket"

elif command -v aria2c &> /dev/null; then
    echo "🚀 检测到 aria2c，正在启动多线程加速下载..."
    aria2c --post-data="password=${PASSWORD}&target=${ENCODED_URL}" "${PROXY_HOST}/api/get-ticket"

else
    echo "❌ 错误: 您的系统未安装 wget、curl 或 aria2c 中的任何一个，请先安装后再运行。"
    exit 1
fi

if [ $? -eq 0 ]; then
    echo -e "\n✨ 下载任务已成功交付！"
else
    echo -e "\n❌ 下载失败，请检查密码或目标服务器网络。"
fi