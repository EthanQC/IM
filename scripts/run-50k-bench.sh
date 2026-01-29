#!/bin/bash
# =============================================================================
# 高并发 WebSocket 压测脚本（50k 连接）
# 用于单台压测机器，配合另一台机器同时运行达到 100k 并发
# 用法: ./run-50k-bench.sh <服务器地址> [用户ID偏移]
# 示例: ./run-50k-bench.sh 192.168.1.100:8084 0
#       ./run-50k-bench.sh 192.168.1.100:8084 50000  # 第二台机器
# =============================================================================

set -e

# 参数
SERVER="${1:-localhost:8084}"
USER_OFFSET="${2:-0}"
CONNS=50000
RAMP="5m"
DURATION="10m"
PING_INTERVAL="45s"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WSBENCH="$SCRIPT_DIR/../bench/wsbench/wsbench"
RESULTS_DIR="$SCRIPT_DIR/../bench/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULT_FILE="$RESULTS_DIR/50k_${TIMESTAMP}.txt"

echo -e "${CYAN}=========================================="
echo "  WebSocket 50k 连接压测"
echo "==========================================${NC}"
echo ""

# 1. 预检查
echo -e "${GREEN}>>> [1/5] 系统预检查${NC}"

# 检查 ulimit
ULIMIT_N=$(ulimit -n)
echo "当前 ulimit -n: $ULIMIT_N"
if [ "$ULIMIT_N" -lt 60000 ]; then
    echo -e "${YELLOW}⚠️  警告: 文件描述符限制太低，尝试提升...${NC}"
    ulimit -n 100000 2>/dev/null || {
        echo -e "${RED}❌ 无法提升 ulimit，请运行: sudo bash $SCRIPT_DIR/tune-bench-client.sh${NC}"
        echo "   然后重新登录终端或执行: ulimit -n 100000"
        exit 1
    }
    echo "已提升 ulimit -n 到: $(ulimit -n)"
fi

# 检查端口范围
if [[ "$OSTYPE" == "darwin"* ]]; then
    PORT_FIRST=$(sysctl -n net.inet.ip.portrange.first 2>/dev/null || echo 49152)
    PORT_LAST=$(sysctl -n net.inet.ip.portrange.last 2>/dev/null || echo 65535)
else
    PORT_RANGE=$(sysctl -n net.ipv4.ip_local_port_range 2>/dev/null || echo "32768 60999")
    PORT_FIRST=$(echo $PORT_RANGE | awk '{print $1}')
    PORT_LAST=$(echo $PORT_RANGE | awk '{print $2}')
fi

AVAIL_PORTS=$((PORT_LAST - PORT_FIRST))
echo "可用端口范围: $PORT_FIRST - $PORT_LAST (共 $AVAIL_PORTS 个)"

if [ "$AVAIL_PORTS" -lt 55000 ]; then
    echo -e "${YELLOW}⚠️  警告: 可用端口数不足，可能无法建立 50k 连接${NC}"
    echo "   建议运行系统调优脚本扩大端口范围"
fi

# 检查 wsbench 是否存在
if [ ! -f "$WSBENCH" ]; then
    echo "编译 wsbench..."
    cd "$SCRIPT_DIR/../bench/wsbench" && go build -o wsbench .
fi

# 检查服务器连接
echo ""
echo "测试服务器连接: $SERVER"
if ! curl -s --connect-timeout 5 "http://$SERVER/health" > /dev/null 2>&1; then
    echo -e "${RED}❌ 无法连接到服务器 $SERVER${NC}"
    exit 1
fi
echo -e "${GREEN}✓ 服务器连接正常${NC}"
echo ""

# 2. 创建结果目录
mkdir -p "$RESULTS_DIR"

# 3. 记录环境信息
echo -e "${GREEN}>>> [2/5] 记录环境信息${NC}"
{
    echo "=== 压测配置 ==="
    echo "时间:       $(date)"
    echo "服务器:     $SERVER"
    echo "连接数:     $CONNS"
    echo "用户偏移:   $USER_OFFSET"
    echo "爬坡时间:   $RAMP"
    echo "持续时间:   $DURATION"
    echo "心跳间隔:   $PING_INTERVAL"
    echo ""
    echo "=== 系统信息 ==="
    echo "主机名:     $(hostname)"
    echo "OS:         $(uname -s) $(uname -r)"
    echo "ulimit -n:  $(ulimit -n)"
    echo "端口范围:   $PORT_FIRST - $PORT_LAST"
    echo ""
} | tee "$RESULT_FILE"

# 4. 显示服务器初始状态
echo -e "${GREEN}>>> [3/5] 服务器初始状态${NC}"
echo "当前连接数: $(curl -s "http://$SERVER/stats" 2>/dev/null | grep -o '"total_connections":[0-9]*' | cut -d':' -f2 || echo 'N/A')"
echo ""

# 5. 运行压测
echo -e "${GREEN}>>> [4/5] 开始压测${NC}"
echo ""
echo "目标: ws://$SERVER/ws"
echo "连接数: $CONNS"
echo "爬坡: $RAMP, 持续: $DURATION"
echo ""

"$WSBENCH" \
    --target="ws://$SERVER/ws" \
    --conns="$CONNS" \
    --duration="$DURATION" \
    --ramp="$RAMP" \
    --ping-interval="$PING_INTERVAL" \
    --mode=connect-only \
    --handshake-timeout=30s \
    --read-buffer=8192 \
    --write-buffer=8192 \
    --max-cps=500 \
    --retry=3 \
    --retry-delay=2s \
    --read-timeout=120s \
    --write-timeout=10s \
    --output=text 2>&1 | tee -a "$RESULT_FILE"

# 6. 记录最终状态
echo "" | tee -a "$RESULT_FILE"
echo -e "${GREEN}>>> [5/5] 最终状态${NC}" | tee -a "$RESULT_FILE"
echo "服务器连接数: $(curl -s "http://$SERVER/stats" 2>/dev/null | grep -o '"total_connections":[0-9]*' | cut -d':' -f2 || echo 'N/A')" | tee -a "$RESULT_FILE"

echo ""
echo -e "${CYAN}=========================================="
echo "  压测完成！"
echo "  结果保存: $RESULT_FILE"
echo -e "==========================================${NC}"
