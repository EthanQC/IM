#!/bin/bash
# =============================================================================
# 压测实时监控脚本 (Docker 版本)
# 用于实时监控 Docker 容器中的 WebSocket 服务器状态
# 用法: ./monitor-bench.sh [服务器地址]
# =============================================================================

SERVER="${1:-localhost:8084}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

clear

while true; do
    clear
    echo -e "${CYAN}=========================================="
    echo "  WebSocket 压测实时监控 (Docker)"
    echo "  服务器: $SERVER"
    echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo -e "==========================================${NC}"
    echo ""
    
    # 1. 服务器连接统计
    echo -e "${GREEN}>>> 服务器连接统计${NC}"
    STATS=$(curl -s --connect-timeout 2 --max-time 3 "http://$SERVER/stats" 2>/dev/null)
    if [ -n "$STATS" ]; then
        TOTAL_CONNS=$(echo "$STATS" | grep -o '"total_connections":[0-9]*' | cut -d':' -f2)
        ONLINE_USERS=$(echo "$STATS" | grep -o '"online_users":[0-9]*' | cut -d':' -f2)
        TOTAL_MSGS=$(echo "$STATS" | grep -o '"total_messages":[0-9]*' | cut -d':' -f2)
        
        echo "  当前连接数:   ${TOTAL_CONNS:-0}"
        echo "  在线用户数:   ${ONLINE_USERS:-0}"
        echo "  消息总数:     ${TOTAL_MSGS:-0}"
    else
        echo -e "  ${RED}无法连接到服务器${NC}"
    fi
    echo ""
    
    # 2. Goroutine 数量
    echo -e "${GREEN}>>> Goroutine 统计${NC}"
    METRICS=$(curl -s --connect-timeout 2 --max-time 3 "http://$SERVER/metrics" 2>/dev/null)
    if [ -n "$METRICS" ]; then
        GOROUTINES=$(echo "$METRICS" | grep "^go_goroutines " | awk '{print $2}')
        if [ -n "$GOROUTINES" ]; then
            echo "  Goroutine 数: $GOROUTINES"
            # 警告阈值
            GOROUTINE_INT=${GOROUTINES%.*}
            if [ "${GOROUTINE_INT:-0}" -gt 200000 ]; then
                echo -e "  ${RED}⚠️  警告: Goroutine 数量过高!${NC}"
            fi
        fi
        
        # 3. 内存使用
        echo ""
        echo -e "${GREEN}>>> 内存统计${NC}"
        HEAP=$(echo "$METRICS" | grep "^go_memstats_heap_inuse_bytes " | awk '{print $2}')
        ALLOC=$(echo "$METRICS" | grep "^go_memstats_alloc_bytes " | awk '{print $2}')
        
        if [ -n "$HEAP" ]; then
            HEAP_MB=$(awk "BEGIN {printf \"%.2f\", $HEAP / 1024 / 1024}")
            echo "  堆内存使用:   ${HEAP_MB} MB"
        fi
        if [ -n "$ALLOC" ]; then
            ALLOC_MB=$(awk "BEGIN {printf \"%.2f\", $ALLOC / 1024 / 1024}")
            echo "  当前分配:     ${ALLOC_MB} MB"
        fi
    else
        echo "  (无法获取 metrics)"
    fi
    echo ""
    
    # 4. Docker 容器状态
    echo -e "${GREEN}>>> Docker 容器状态${NC}"
    if command -v docker &> /dev/null; then
        DELIVERY_STATUS=$(docker ps --filter "name=im_delivery" --format "{{.Status}}" 2>/dev/null)
        if [ -n "$DELIVERY_STATUS" ]; then
            echo "  im_delivery:  $DELIVERY_STATUS"
            
            # 容器内存使用
            CONTAINER_MEM=$(docker stats im_delivery --no-stream --format "{{.MemUsage}}" 2>/dev/null)
            if [ -n "$CONTAINER_MEM" ]; then
                echo "  容器内存:     $CONTAINER_MEM"
            fi
            
            # 容器 CPU
            CONTAINER_CPU=$(docker stats im_delivery --no-stream --format "{{.CPUPerc}}" 2>/dev/null)
            if [ -n "$CONTAINER_CPU" ]; then
                echo "  容器 CPU:     $CONTAINER_CPU"
            fi
        else
            echo -e "  ${YELLOW}im_delivery 容器未运行${NC}"
        fi
    else
        echo "  (Docker 未安装)"
    fi
    echo ""
    
    # 5. 系统资源
    echo -e "${GREEN}>>> 系统资源${NC}"
    
    # macOS ulimit
    ULIMIT_N=$(ulimit -n 2>/dev/null)
    echo "  ulimit -n:    $ULIMIT_N"
    
    # CPU 负载
    if [[ "$OSTYPE" == "darwin"* ]]; then
        LOAD=$(sysctl -n vm.loadavg 2>/dev/null | tr -d '{}')
        echo "  系统负载:     $LOAD"
    else
        LOAD=$(uptime | awk -F'load average:' '{print $2}' | xargs)
        echo "  系统负载:     $LOAD"
    fi
    echo ""
    
    # 6. 宿主机网络连接（简化版，避免卡顿）
    echo -e "${GREEN}>>> 宿主机 8084 端口连接${NC}"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS: 使用 netstat 快速统计
        ESTABLISHED=$(netstat -an 2>/dev/null | grep -c "\.8084.*ESTABLISHED" || echo "0")
        echo "  ESTABLISHED:  $ESTABLISHED"
    else
        # Linux: 使用 ss
        if command -v ss &> /dev/null; then
            ESTABLISHED=$(ss -tn state established "( sport = :8084 )" 2>/dev/null | wc -l)
            ESTABLISHED=$((ESTABLISHED - 1))
            echo "  ESTABLISHED:  $ESTABLISHED"
        fi
    fi
    echo ""
    
    echo -e "${BLUE}按 Ctrl+C 退出 | 刷新间隔: 3秒${NC}"
    
    sleep 3
done
