#!/bin/bash
# =============================================================================
# 压测实时监控脚本
# 用于实时监控 WebSocket 服务器和压测客户端的状态
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
    echo "  WebSocket 压测实时监控"
    echo "  服务器: $SERVER"
    echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo -e "==========================================${NC}"
    echo ""
    
    # 1. 服务器连接统计
    echo -e "${GREEN}>>> 服务器连接统计${NC}"
    STATS=$(curl -s "http://$SERVER/stats" 2>/dev/null)
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
    GOROUTINES=$(curl -s "http://$SERVER/metrics" 2>/dev/null | grep "^go_goroutines " | awk '{print $2}')
    if [ -n "$GOROUTINES" ]; then
        echo "  Goroutine 数: $GOROUTINES"
        
        # 警告阈值
        if [ "${GOROUTINES%.*}" -gt 200000 ]; then
            echo -e "  ${RED}⚠️  警告: Goroutine 数量过高!${NC}"
        fi
    fi
    echo ""
    
    # 3. 内存使用
    echo -e "${GREEN}>>> 内存统计${NC}"
    HEAP=$(curl -s "http://$SERVER/metrics" 2>/dev/null | grep "^go_memstats_heap_inuse_bytes " | awk '{print $2}')
    if [ -n "$HEAP" ]; then
        HEAP_MB=$(echo "scale=2; $HEAP / 1024 / 1024" | bc 2>/dev/null || echo "N/A")
        echo "  堆内存使用:   ${HEAP_MB} MB"
    fi
    
    ALLOC=$(curl -s "http://$SERVER/metrics" 2>/dev/null | grep "^go_memstats_alloc_bytes " | awk '{print $2}')
    if [ -n "$ALLOC" ]; then
        ALLOC_MB=$(echo "scale=2; $ALLOC / 1024 / 1024" | bc 2>/dev/null || echo "N/A")
        echo "  当前分配:     ${ALLOC_MB} MB"
    fi
    echo ""
    
    # 4. 本机网络连接统计
    echo -e "${GREEN}>>> 本机网络统计${NC}"
    
    # 端口 8084 的连接状态
    if command -v ss &> /dev/null; then
        # Linux
        ESTABLISHED=$(ss -tn state established "( sport = :8084 )" 2>/dev/null | wc -l)
        ESTABLISHED=$((ESTABLISHED - 1)) # 减去标题行
        TIME_WAIT=$(ss -tn state time-wait "( sport = :8084 )" 2>/dev/null | wc -l)
        TIME_WAIT=$((TIME_WAIT - 1))
    else
        # macOS
        ESTABLISHED=$(netstat -an 2>/dev/null | grep "\.8084 " | grep ESTABLISHED | wc -l | tr -d ' ')
        TIME_WAIT=$(netstat -an 2>/dev/null | grep "\.8084 " | grep TIME_WAIT | wc -l | tr -d ' ')
    fi
    
    echo "  ESTABLISHED:  ${ESTABLISHED:-0}"
    echo "  TIME_WAIT:    ${TIME_WAIT:-0}"
    echo ""
    
    # 5. 系统资源
    echo -e "${GREEN}>>> 系统资源${NC}"
    
    # 文件描述符使用
    if [ -f /proc/sys/fs/file-nr ]; then
        FD_INFO=$(cat /proc/sys/fs/file-nr)
        FD_USED=$(echo $FD_INFO | awk '{print $1}')
        FD_MAX=$(echo $FD_INFO | awk '{print $3}')
        echo "  文件描述符:   $FD_USED / $FD_MAX"
    else
        # macOS
        ULIMIT_N=$(ulimit -n 2>/dev/null)
        echo "  ulimit -n:    $ULIMIT_N"
    fi
    
    # CPU 负载
    LOAD=$(uptime | awk -F'load average:' '{print $2}' | xargs)
    echo "  系统负载:     $LOAD"
    echo ""
    
    # 6. 进程状态（如果 delivery 服务在运行）
    echo -e "${GREEN}>>> 服务进程${NC}"
    
    if pgrep -f "delivery" > /dev/null 2>&1; then
        PID=$(pgrep -f "delivery_service" | head -1)
        if [ -n "$PID" ]; then
            if [ -d "/proc/$PID" ]; then
                # Linux
                FD_COUNT=$(ls /proc/$PID/fd 2>/dev/null | wc -l)
                echo "  进程 PID:     $PID"
                echo "  打开 FD 数:   $FD_COUNT"
            else
                # macOS
                echo "  进程 PID:     $PID"
            fi
        fi
    else
        echo -e "  ${YELLOW}delivery_service 进程未找到${NC}"
    fi
    echo ""
    
    echo -e "${BLUE}按 Ctrl+C 退出${NC}"
    
    sleep 2
done
