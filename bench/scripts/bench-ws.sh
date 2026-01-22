#!/bin/bash
# WebSocket 连接压测脚本（本地运行版）
# 用法: ./bench-ws.sh <连接数> [持续时间] [爬坡时间]
# 示例: ./bench-ws.sh 1000 2m 30s

set -e

# 参数解析
CONNS="${1:-1000}"
DURATION="${2:-2m}"
RAMP="${3:-30s}"
TARGET="${TARGET:-ws://localhost:30084/ws}"

# 颜色定义
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WSBENCH_DIR="$(dirname "$SCRIPT_DIR")/wsbench"
RESULTS_DIR="$(dirname "$SCRIPT_DIR")/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULT_FILE="$RESULTS_DIR/ws_${CONNS}_${TIMESTAMP}.txt"

# 确保 wsbench 已编译
if [ ! -f "$WSBENCH_DIR/wsbench" ]; then
    echo -e "${YELLOW}>>> 编译 wsbench...${NC}"
    cd "$WSBENCH_DIR" && go build -o wsbench .
fi

# 创建结果目录
mkdir -p "$RESULTS_DIR"

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}   WebSocket 连接压测 - ${CONNS} 连接${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo "目标地址: $TARGET"
echo "连接数:   $CONNS"
echo "持续时间: $DURATION"
echo "爬坡时间: $RAMP"
echo "开始时间: $(date)"
echo ""

# 记录环境信息
{
    echo "=== 环境信息 ==="
    echo "测试时间: $(date)"
    echo "目标地址: $TARGET"
    echo "连接数: $CONNS"
    echo "持续时间: $DURATION"
    echo "爬坡时间: $RAMP"
    echo ""
    echo "=== 系统信息 ==="
    echo "OS: $(uname -s) $(uname -r)"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "CPU: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'N/A')"
        echo "核心数: $(sysctl -n hw.ncpu 2>/dev/null || echo 'N/A')"
        echo "内存: $(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 / 1024 )) GB"
    fi
    echo ""
    echo "=== K8s 状态 ==="
    kubectl get pods -n im 2>/dev/null || echo "K8s 不可用"
    echo ""
    echo "=== 资源使用 ==="
    kubectl top pods -n im 2>/dev/null || echo "metrics-server 不可用"
    echo ""
    echo "=========================================="
    echo ""
} > "$RESULT_FILE"

# 运行压测
echo -e "${GREEN}>>> 开始压测...${NC}"
echo ""

"$WSBENCH_DIR/wsbench" \
    --target="$TARGET" \
    --conns="$CONNS" \
    --duration="$DURATION" \
    --ramp="$RAMP" \
    --mode=connect-only \
    --output=text 2>&1 | tee -a "$RESULT_FILE"

echo ""
echo -e "${GREEN}>>> 压测完成${NC}"
echo "结果保存: $RESULT_FILE"
echo ""

# 收集结束时资源状态
{
    echo ""
    echo "=== 压测后资源状态 ==="
    kubectl top pods -n im 2>/dev/null || echo "metrics-server 不可用"
} >> "$RESULT_FILE"
