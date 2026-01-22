#!/bin/bash
# 消息吞吐量压测脚本
# 用法: ./bench-msg.sh <connections> <msg_rate> <duration> <ramp_duration>
# 示例: ./bench-msg.sh 5000 10 5m 1m

set -e

# 参数解析
CONNECTIONS="${1:-5000}"
MSG_RATE="${2:-10}"        # 每连接每秒消息数
DURATION="${3:-5m}"
RAMP_DURATION="${4:-1m}"
NAMESPACE="${NAMESPACE:-im}"

# 计算 Pod 配置
CONNS_PER_POD=1000
PODS=$(( (CONNECTIONS + CONNS_PER_POD - 1) / CONNS_PER_POD ))

# 颜色定义
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$(dirname "$SCRIPT_DIR")/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_DIR="$RESULTS_DIR/msg_${CONNECTIONS}c_${MSG_RATE}mps_${TIMESTAMP}"

echo -e "${CYAN}=== 消息吞吐量压测 ===${NC}"
echo ""
echo "测试参数:"
echo "  连接数:         $CONNECTIONS"
echo "  消息速率:       ${MSG_RATE} msg/s per connection"
echo "  总速率:         $((CONNECTIONS * MSG_RATE)) msg/s"
echo "  持续时间:       $DURATION"
echo "  爬坡时间:       $RAMP_DURATION"
echo "  压测 Pod 数:    $PODS"
echo "  每 Pod 连接数:  $CONNS_PER_POD"
echo "  命名空间:       $NAMESPACE"
echo ""

# 创建结果目录
mkdir -p "$TEST_DIR"

# 记录环境信息
echo -e "${GREEN}>>> 记录环境信息...${NC}"
cat > "$TEST_DIR/test-config.txt" << EOF
=== 消息吞吐量压测配置 ===
测试时间: $(date)
测试模式: messaging

--- 压测参数 ---
连接数:         $CONNECTIONS
消息速率:       ${MSG_RATE} msg/s per connection
总消息速率:     $((CONNECTIONS * MSG_RATE)) msg/s
持续时间:       $DURATION
爬坡时间:       $RAMP_DURATION
压测 Pod 数:    $PODS
每 Pod 连接数:  $CONNS_PER_POD

--- K8s 环境 ---
集群版本: $(kubectl version --short 2>/dev/null | grep Server | cut -d' ' -f3)
命名空间: $NAMESPACE

--- 节点信息 ---
$(kubectl get nodes -o wide)

--- 当前资源使用 ---
$(kubectl top nodes 2>/dev/null || echo "metrics-server 不可用")
EOF

echo ""
echo -e "${GREEN}>>> 检查 wsbench 镜像...${NC}"
if ! docker images im/wsbench:latest -q | grep -q .; then
    echo -e "${YELLOW}wsbench 镜像不存在，开始构建...${NC}"
    make -C "$(dirname "$(dirname "$SCRIPT_DIR")")" build-wsbench
fi

echo ""
echo -e "${GREEN}>>> 配置 wsbench Deployment（messaging 模式）...${NC}"
kubectl set env deployment/wsbench -n "$NAMESPACE" \
    BENCH_MODE=messaging \
    CONNS_PER_POD="$CONNS_PER_POD" \
    MSG_RATE="$MSG_RATE" \
    DURATION="$DURATION" \
    RAMP_DURATION="$RAMP_DURATION" \
    > /dev/null 2>&1

echo -e "${GREEN}>>> 扩容 wsbench 至 ${PODS} 个 Pod...${NC}"
kubectl scale deployment/wsbench -n "$NAMESPACE" --replicas="$PODS" > /dev/null 2>&1

echo ""
echo -e "${GREEN}>>> 等待 wsbench Pod 就绪...${NC}"
sleep 5
kubectl wait --for=condition=ready pod -l app=wsbench -n "$NAMESPACE" --timeout=120s || {
    echo -e "${RED}wsbench Pod 启动超时${NC}"
    kubectl get pods -n "$NAMESPACE" -l app=wsbench
    exit 1
}

echo ""
echo -e "${GREEN}>>> 消息压测运行中...${NC}"
echo ""
echo "预计吞吐量: $((CONNECTIONS * MSG_RATE)) msg/s"
echo ""
echo "监控命令:"
echo "  kubectl get pods -n $NAMESPACE -l app=wsbench -w"
echo "  kubectl logs -f -l app=wsbench -n $NAMESPACE --tail=50"
echo "  kubectl top pods -n $NAMESPACE"
echo ""
echo "停止压测: make bench-stop"
echo ""

# 记录开始时间
START_TIME=$(date +%s)
echo "开始时间: $(date)" > "$TEST_DIR/timing.txt"

# 计算持续时间（转换为秒）
DURATION_SEC=$(echo "$DURATION" | sed 's/m/*60/;s/s//;s/h/*3600/' | bc 2>/dev/null || echo 300)
RAMP_SEC=$(echo "$RAMP_DURATION" | sed 's/m/*60/;s/s//;s/h/*3600/' | bc 2>/dev/null || echo 60)
TOTAL_WAIT_SEC=$((DURATION_SEC + RAMP_SEC + 30))

echo -e "${YELLOW}压测将持续约 $((TOTAL_WAIT_SEC / 60)) 分钟...${NC}"
echo ""

# 定期采样
SAMPLE_INTERVAL=30
ELAPSED=0
LAST_MSG_COUNT=0

while [ $ELAPSED -lt $TOTAL_WAIT_SEC ]; do
    sleep $SAMPLE_INTERVAL
    ELAPSED=$((ELAPSED + SAMPLE_INTERVAL))
    
    echo -e "${CYAN}[$(date +%H:%M:%S)] 采样 ($ELAPSED/$TOTAL_WAIT_SEC 秒)${NC}"
    
    # 检查 Pod 状态
    RUNNING_PODS=$(kubectl get pods -n "$NAMESPACE" -l app=wsbench --no-headers 2>/dev/null | grep -c Running || echo 0)
    echo "  运行中的 Pod: $RUNNING_PODS/$PODS"
    
    # 统计当前消息数
    CURRENT_MSG_COUNT=0
    for pod in $(kubectl get pods -n "$NAMESPACE" -l app=wsbench -o jsonpath='{.items[*].metadata.name}'); do
        MSG_SENT=$(kubectl logs "$pod" -n "$NAMESPACE" --tail=20 2>/dev/null | grep -oP '(?<=messages_sent":)\d+' | tail -1 || echo 0)
        CURRENT_MSG_COUNT=$((CURRENT_MSG_COUNT + MSG_SENT))
    done
    
    # 计算速率
    if [ $LAST_MSG_COUNT -gt 0 ]; then
        MSG_RATE_ACTUAL=$(( (CURRENT_MSG_COUNT - LAST_MSG_COUNT) / SAMPLE_INTERVAL ))
        echo "  当前吞吐: $MSG_RATE_ACTUAL msg/s (目标: $((CONNECTIONS * MSG_RATE)) msg/s)"
    fi
    LAST_MSG_COUNT=$CURRENT_MSG_COUNT
    
    # 采样资源使用
    if kubectl top pods -n "$NAMESPACE" -l app=delivery-service &>/dev/null; then
        echo "  Delivery Service 资源:"
        kubectl top pods -n "$NAMESPACE" -l app=delivery-service --no-headers | head -3
    fi
    
    # 检查是否完成
    COMPLETED=$(kubectl get pods -n "$NAMESPACE" -l app=wsbench --no-headers 2>/dev/null | grep -c Completed || echo 0)
    if [ "$COMPLETED" -eq "$PODS" ]; then
        echo -e "${GREEN}所有 Pod 已完成${NC}"
        break
    fi
    
    echo ""
done

# 记录结束时间
END_TIME=$(date +%s)
echo "结束时间: $(date)" >> "$TEST_DIR/timing.txt"
echo "总耗时: $((END_TIME - START_TIME)) 秒" >> "$TEST_DIR/timing.txt"

echo ""
echo -e "${GREEN}>>> 收集测试结果...${NC}"

# 收集日志
LOG_DIR="$TEST_DIR/wsbench-logs"
mkdir -p "$LOG_DIR"

for pod in $(kubectl get pods -n "$NAMESPACE" -l app=wsbench -o jsonpath='{.items[*].metadata.name}'); do
    echo "  收集 $pod 日志..."
    kubectl logs "$pod" -n "$NAMESPACE" > "$LOG_DIR/$pod.log" 2>&1 || true
done

# 分析结果
echo ""
echo -e "${GREEN}>>> 分析测试结果...${NC}"

SUMMARY_FILE="$TEST_DIR/summary.txt"
cat > "$SUMMARY_FILE" << EOF
=== 消息吞吐量压测结果 ===
测试时间: $(date)
配置: $CONNECTIONS 连接 × ${MSG_RATE} msg/s = $((CONNECTIONS * MSG_RATE)) msg/s

--- 测试时长 ---
$(cat "$TEST_DIR/timing.txt")

--- Pod 状态 ---
$(kubectl get pods -n "$NAMESPACE" -l app=wsbench)

--- 消息统计 ---
EOF

# 解析消息统计
TOTAL_SENT=0
TOTAL_RECEIVED=0
TOTAL_FAILED=0
declare -a LATENCIES

for log in "$LOG_DIR"/*.log; do
    if [ -f "$log" ]; then
        SENT=$(grep -oP '(?<=messages_sent":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        RECEIVED=$(grep -oP '(?<=messages_received":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        FAILED=$(grep -oP '(?<=messages_failed":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        
        TOTAL_SENT=$((TOTAL_SENT + SENT))
        TOTAL_RECEIVED=$((TOTAL_RECEIVED + RECEIVED))
        TOTAL_FAILED=$((TOTAL_FAILED + FAILED))
        
        # 提取延迟数据
        P50=$(grep -oP '(?<="p50_ms":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        P95=$(grep -oP '(?<="p95_ms":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        P99=$(grep -oP '(?<="p99_ms":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        
        if [ "$P50" -gt 0 ]; then
            LATENCIES+=("$P50 $P95 $P99")
        fi
    fi
done

# 计算平均吞吐
ACTUAL_DURATION=$((END_TIME - START_TIME - RAMP_SEC))
if [ $ACTUAL_DURATION -gt 0 ]; then
    AVG_THROUGHPUT=$((TOTAL_SENT / ACTUAL_DURATION))
else
    AVG_THROUGHPUT=0
fi

cat >> "$SUMMARY_FILE" << EOF
总发送消息数:   $TOTAL_SENT
总接收消息数:   $TOTAL_RECEIVED
失败消息数:     $TOTAL_FAILED
投递成功率:     $(awk "BEGIN {printf \"%.2f\", ($TOTAL_RECEIVED / ($TOTAL_SENT + 0.0001)) * 100}")%
平均吞吐量:     $AVG_THROUGHPUT msg/s
目标吞吐量:     $((CONNECTIONS * MSG_RATE)) msg/s
达成率:         $(awk "BEGIN {printf \"%.2f\", ($AVG_THROUGHPUT / ($CONNECTIONS * MSG_RATE + 0.0001)) * 100}")%

--- 延迟统计 (ms) ---
EOF

# 计算平均延迟
if [ ${#LATENCIES[@]} -gt 0 ]; then
    SUM_P50=0
    SUM_P95=0
    SUM_P99=0
    COUNT=0
    
    for lat in "${LATENCIES[@]}"; do
        read P50 P95 P99 <<< "$lat"
        SUM_P50=$((SUM_P50 + P50))
        SUM_P95=$((SUM_P95 + P95))
        SUM_P99=$((SUM_P99 + P99))
        COUNT=$((COUNT + 1))
    done
    
    cat >> "$SUMMARY_FILE" << EOF
P50 (中位数):   $((SUM_P50 / COUNT)) ms
P95:            $((SUM_P95 / COUNT)) ms
P99:            $((SUM_P99 / COUNT)) ms
EOF
else
    echo "延迟数据未采集" >> "$SUMMARY_FILE"
fi

cat >> "$SUMMARY_FILE" << EOF

--- 资源使用峰值 ---
$(kubectl top pods -n "$NAMESPACE" -l app=delivery-service 2>/dev/null || echo "metrics-server 不可用")

--- HPA 状态 ---
$(kubectl get hpa -n "$NAMESPACE" 2>/dev/null || echo "HPA 未配置")
EOF

# 停止压测
echo ""
echo -e "${YELLOW}>>> 停止压测并清理...${NC}"
kubectl scale deployment/wsbench -n "$NAMESPACE" --replicas=0 > /dev/null 2>&1

echo ""
echo -e "${GREEN}=== 压测完成 ===${NC}"
echo ""
echo "结果目录: $TEST_DIR"
echo ""
cat "$SUMMARY_FILE"
echo ""

# 性能警告
DELIVERY_RATE=$(awk "BEGIN {printf \"%.0f\", ($TOTAL_RECEIVED / ($TOTAL_SENT + 0.0001)) * 100}")
if [ "$DELIVERY_RATE" -lt 95 ]; then
    echo -e "${RED}警告: 消息投递率 ${DELIVERY_RATE}% 低于预期 95%${NC}"
    echo "可能原因:"
    echo "  1. Kafka 消费延迟"
    echo "  2. Delivery Service 资源不足"
    echo "  3. 消息速率超过系统处理能力"
    echo ""
fi

THROUGHPUT_RATE=$(awk "BEGIN {printf \"%.0f\", ($AVG_THROUGHPUT / ($CONNECTIONS * MSG_RATE + 0.0001)) * 100}")
if [ "$THROUGHPUT_RATE" -lt 80 ]; then
    echo -e "${YELLOW}注意: 实际吞吐量 ${THROUGHPUT_RATE}% 达成目标${NC}"
    echo "可能原因:"
    echo "  1. 客户端压力不足"
    echo "  2. 网络带宽限制"
    echo "  3. 目标设置过高"
    echo ""
fi

echo "建议运行完整数据采集: make bench-collect"
echo ""

exit 0
