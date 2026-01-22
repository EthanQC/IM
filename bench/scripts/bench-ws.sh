#!/bin/bash
# WebSocket 连接压测脚本
# 用法: ./bench-ws.sh <total_conns> <pods> <conns_per_pod> <duration> <ramp_duration>
# 示例: ./bench-ws.sh 10000 10 1000 5m 1m

set -e

# 参数解析
TOTAL_CONNS="${1:-10000}"
PODS="${2:-10}"
CONNS_PER_POD="${3:-1000}"
DURATION="${4:-5m}"
RAMP_DURATION="${5:-1m}"
NAMESPACE="${NAMESPACE:-im}"

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
TEST_DIR="$RESULTS_DIR/ws_${TOTAL_CONNS}_${TIMESTAMP}"

echo -e "${CYAN}=== WebSocket 连接压测 ===${NC}"
echo ""
echo "测试参数:"
echo "  目标连接数:     $TOTAL_CONNS"
echo "  压测 Pod 数:    $PODS"
echo "  每 Pod 连接数:  $CONNS_PER_POD"
echo "  持续时间:       $DURATION"
echo "  爬坡时间:       $RAMP_DURATION"
echo "  命名空间:       $NAMESPACE"
echo ""

# 创建结果目录
mkdir -p "$TEST_DIR"

# 记录环境信息
echo -e "${GREEN}>>> 记录环境信息...${NC}"
cat > "$TEST_DIR/test-config.txt" << EOF
=== WebSocket 连接压测配置 ===
测试时间: $(date)
测试模式: connect-only

--- 压测参数 ---
目标连接数:     $TOTAL_CONNS
压测 Pod 数:    $PODS
每 Pod 连接数:  $CONNS_PER_POD
持续时间:       $DURATION
爬坡时间:       $RAMP_DURATION

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
echo -e "${GREEN}>>> 配置 wsbench Deployment...${NC}"
kubectl set env deployment/wsbench -n "$NAMESPACE" \
    BENCH_MODE=connect-only \
    CONNS_PER_POD="$CONNS_PER_POD" \
    DURATION="$DURATION" \
    RAMP_DURATION="$RAMP_DURATION" \
    > /dev/null 2>&1

echo -e "${GREEN}>>> 扩容 wsbench 至 ${PODS} 个 Pod...${NC}"
kubectl scale deployment/wsbench -n "$NAMESPACE" --replicas="$PODS" > /dev/null 2>&1

echo ""
echo -e "${GREEN}>>> 等待 wsbench Pod 就绪...${NC}"
# 等待 Pod 启动
sleep 5
kubectl wait --for=condition=ready pod -l app=wsbench -n "$NAMESPACE" --timeout=120s || {
    echo -e "${RED}wsbench Pod 启动超时${NC}"
    kubectl get pods -n "$NAMESPACE" -l app=wsbench
    exit 1
}

echo ""
echo -e "${GREEN}>>> 压测运行中...${NC}"
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
# 添加爬坡时间和缓冲时间
RAMP_SEC=$(echo "$RAMP_DURATION" | sed 's/m/*60/;s/s//;s/h/*3600/' | bc 2>/dev/null || echo 60)
TOTAL_WAIT_SEC=$((DURATION_SEC + RAMP_SEC + 30))

echo -e "${YELLOW}压测将持续约 $((TOTAL_WAIT_SEC / 60)) 分钟...${NC}"
echo ""

# 定期采样
SAMPLE_INTERVAL=30
ELAPSED=0
while [ $ELAPSED -lt $TOTAL_WAIT_SEC ]; do
    sleep $SAMPLE_INTERVAL
    ELAPSED=$((ELAPSED + SAMPLE_INTERVAL))
    
    # 采样当前状态
    echo -e "${CYAN}[$(date +%H:%M:%S)] 采样 ($ELAPSED/$TOTAL_WAIT_SEC 秒)${NC}"
    
    # 检查 Pod 状态
    RUNNING_PODS=$(kubectl get pods -n "$NAMESPACE" -l app=wsbench --no-headers 2>/dev/null | grep -c Running || echo 0)
    echo "  运行中的 Pod: $RUNNING_PODS/$PODS"
    
    # 采样资源使用
    if kubectl top pods -n "$NAMESPACE" -l app=wsbench &>/dev/null; then
        kubectl top pods -n "$NAMESPACE" -l app=wsbench --no-headers | head -5
    fi
    
    # 检查是否所有 Pod 都完成了
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

# 收集所有 wsbench Pod 的日志
LOG_DIR="$TEST_DIR/wsbench-logs"
mkdir -p "$LOG_DIR"

for pod in $(kubectl get pods -n "$NAMESPACE" -l app=wsbench -o jsonpath='{.items[*].metadata.name}'); do
    echo "  收集 $pod 日志..."
    kubectl logs "$pod" -n "$NAMESPACE" > "$LOG_DIR/$pod.log" 2>&1 || true
done

# 汇总结果
echo ""
echo -e "${GREEN}>>> 分析测试结果...${NC}"

# 从日志中提取关键指标
SUMMARY_FILE="$TEST_DIR/summary.txt"
cat > "$SUMMARY_FILE" << EOF
=== WebSocket 连接压测结果 ===
测试时间: $(date)
配置: $TOTAL_CONNS 连接 = $PODS Pod × $CONNS_PER_POD

--- 测试时长 ---
$(cat "$TEST_DIR/timing.txt")

--- Pod 状态 ---
$(kubectl get pods -n "$NAMESPACE" -l app=wsbench)

--- 连接统计 ---
EOF

# 解析所有日志，提取成功率等指标
TOTAL_SUCCESS=0
TOTAL_FAILED=0
for log in "$LOG_DIR"/*.log; do
    if [ -f "$log" ]; then
        SUCCESS=$(grep -oP '(?<=success_conns":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        FAILED=$(grep -oP '(?<=failed_conns":)\d+' "$log" 2>/dev/null | tail -1 || echo 0)
        TOTAL_SUCCESS=$((TOTAL_SUCCESS + SUCCESS))
        TOTAL_FAILED=$((TOTAL_FAILED + FAILED))
    fi
done

cat >> "$SUMMARY_FILE" << EOF
成功连接数: $TOTAL_SUCCESS
失败连接数: $TOTAL_FAILED
成功率: $(awk "BEGIN {printf \"%.2f\", ($TOTAL_SUCCESS / ($TOTAL_SUCCESS + $TOTAL_FAILED + 0.0001)) * 100}")%

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

# 如果成功率低于 95%，给出警告
SUCCESS_RATE=$(awk "BEGIN {printf \"%.0f\", ($TOTAL_SUCCESS / ($TOTAL_SUCCESS + $TOTAL_FAILED + 0.0001)) * 100}")
if [ "$SUCCESS_RATE" -lt 95 ]; then
    echo -e "${RED}警告: 连接成功率 ${SUCCESS_RATE}% 低于预期 95%${NC}"
    echo "可能原因:"
    echo "  1. 资源不足（CPU/内存）"
    echo "  2. 网络限制（文件描述符）"
    echo "  3. 爬坡时间过短"
    echo ""
    echo "建议:"
    echo "  - 查看详细日志: ls $LOG_DIR"
    echo "  - 运行完整采集: make bench-collect"
    echo "  - 减少目标连接数重试"
fi

exit 0
