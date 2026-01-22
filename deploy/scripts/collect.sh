#!/bin/bash
# 压测数据收集脚本 - 完整版
# 用法: ./collect.sh [namespace] [result_dir]
# 收集 K8s 环境信息、资源使用、日志、指标等，用于压测结果分析和问题排查

set -e

NAMESPACE="${1:-im}"
RESULT_DIR="${2:-$(dirname "$0")/../../bench/results/collect_$(date +%Y%m%d_%H%M%S)}"

# 颜色定义
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

mkdir -p "$RESULT_DIR"

echo -e "${CYAN}=== 压测数据收集 ===${NC}"
echo "命名空间: $NAMESPACE"
echo "结果目录: $RESULT_DIR"
echo "开始时间: $(date)"
echo ""

# 创建收集信息文件
COLLECT_INFO="$RESULT_DIR/collection-info.txt"
cat > "$COLLECT_INFO" << EOF
=== 数据收集信息 ===
收集时间: $(date)
命名空间: $NAMESPACE
收集脚本: $0
执行用户: $(whoami)
主机名: $(hostname)

EOF

# 1. 环境信息
echo -e "${GREEN}>>> [1/12] 收集环境信息...${NC}"
{
    echo "=== 系统环境 ==="
    echo "操作系统: $(uname -s)"
    echo "内核版本: $(uname -r)"
    echo "架构: $(uname -m)"
    echo ""
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "=== macOS 信息 ==="
        sw_vers 2>/dev/null || true
        echo ""
        echo "CPU 信息:"
        sysctl -n machdep.cpu.brand_string 2>/dev/null || true
        echo "CPU 核心数: $(sysctl -n hw.ncpu 2>/dev/null || echo "未知")"
        echo "物理内存: $(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 / 1024 )) GB"
    elif [[ "$OSTYPE" == "linux"* ]]; then
        echo "=== Linux 信息 ==="
        cat /etc/os-release 2>/dev/null | head -5 || true
        echo ""
        echo "CPU 信息:"
        lscpu 2>/dev/null | grep -E "^(Architecture|CPU|Model name|Thread|Core)" || true
        echo ""
        echo "内存信息:"
        free -h 2>/dev/null || true
    fi
} > "$RESULT_DIR/environment.txt" 2>&1

# 2. 集群信息
echo -e "${GREEN}>>> [2/12] 收集 K8s 集群信息...${NC}"
{
    echo "=== Kubernetes 版本 ==="
    kubectl version --short 2>/dev/null || kubectl version 2>/dev/null || true
    echo ""
    echo "=== 集群信息 ==="
    kubectl cluster-info 2>&1
    echo ""
    echo "=== API 资源 ==="
    kubectl api-resources --verbs=list --namespaced -o name 2>/dev/null | head -20 || true
} > "$RESULT_DIR/cluster-info.txt" 2>&1

# 3. 节点信息
echo -e "${GREEN}>>> [3/12] 收集节点信息...${NC}"
{
    echo "=== 节点列表 ==="
    kubectl get nodes -o wide
    echo ""
    echo "=== 节点详细信息 ==="
    for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
        echo ""
        echo "--- Node: $node ---"
        kubectl describe node "$node" 2>/dev/null | head -100 || true
    done
} > "$RESULT_DIR/nodes.txt" 2>&1

# 4. Pod 状态
echo -e "${GREEN}>>> [4/12] 收集 Pod 状态...${NC}"
kubectl get pods -n "$NAMESPACE" -o wide > "$RESULT_DIR/pods.txt" 2>&1 || true
kubectl get pods -n "$NAMESPACE" -o yaml > "$RESULT_DIR/pods.yaml" 2>&1 || true

# 5. Service 和 Endpoint
echo -e "${GREEN}>>> [5/12] 收集 Service 信息...${NC}"
{
    echo "=== Services ==="
    kubectl get svc -n "$NAMESPACE" -o wide
    echo ""
    echo "=== Endpoints ==="
    kubectl get endpoints -n "$NAMESPACE"
} > "$RESULT_DIR/services.txt" 2>&1 || true

# 6. HPA 状态
echo -e "${GREEN}>>> [6/12] 收集 HPA 状态...${NC}"
{
    echo "=== HPA 列表 ==="
    kubectl get hpa -n "$NAMESPACE"
    echo ""
    echo "=== HPA 详细信息 ==="
    for hpa in $(kubectl get hpa -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
        echo ""
        echo "--- HPA: $hpa ---"
        kubectl describe hpa "$hpa" -n "$NAMESPACE" 2>/dev/null || true
    done
} > "$RESULT_DIR/hpa.txt" 2>&1 || true

# 7. 资源使用（需要 metrics-server）
echo -e "${GREEN}>>> [7/12] 收集资源使用...${NC}"
if kubectl top nodes &>/dev/null; then
    {
        echo "=== 节点资源使用 ==="
        kubectl top nodes
        echo ""
        echo "=== Pod 资源使用 (所有命名空间) ==="
        kubectl top pods -n "$NAMESPACE" --containers 2>/dev/null || kubectl top pods -n "$NAMESPACE"
        echo ""
        echo "=== API Gateway 资源 ==="
        kubectl top pods -n "$NAMESPACE" -l app=api-gateway 2>/dev/null || true
        echo ""
        echo "=== Delivery Service 资源 ==="
        kubectl top pods -n "$NAMESPACE" -l app=delivery-service 2>/dev/null || true
        echo ""
        echo "=== wsbench 资源 ==="
        kubectl top pods -n "$NAMESPACE" -l app=wsbench 2>/dev/null || true
    } > "$RESULT_DIR/resource-usage.txt" 2>&1
else
    echo "metrics-server 未安装或不可用" > "$RESULT_DIR/resource-usage.txt"
    echo "  运行 'make install-metrics-server' 安装" >> "$RESULT_DIR/resource-usage.txt"
fi

# 8. Pod 描述
echo -e "${GREEN}>>> [8/12] 收集 Pod 详细描述...${NC}"
DESCRIBE_DIR="$RESULT_DIR/describe"
mkdir -p "$DESCRIBE_DIR"
for pod in $(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}'); do
    kubectl describe pod "$pod" -n "$NAMESPACE" > "$DESCRIBE_DIR/$pod.txt" 2>&1 || true
done

# 9. 日志收集
echo -e "${GREEN}>>> [9/12] 收集 Pod 日志...${NC}"
LOG_DIR="$RESULT_DIR/logs"
mkdir -p "$LOG_DIR"

# API Gateway 日志
echo "  - API Gateway..."
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=api-gateway -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=2000 > "$LOG_DIR/$pod.log" 2>&1 || true
    # 收集之前的日志（如果 Pod 重启过）
    kubectl logs "$pod" -n "$NAMESPACE" --previous --tail=1000 > "$LOG_DIR/$pod-previous.log" 2>&1 || true
done

# Delivery Service 日志
echo "  - Delivery Service..."
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=delivery-service -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=2000 > "$LOG_DIR/$pod.log" 2>&1 || true
    kubectl logs "$pod" -n "$NAMESPACE" --previous --tail=1000 > "$LOG_DIR/$pod-previous.log" 2>&1 || true
done

# wsbench 日志
echo "  - wsbench..."
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=wsbench -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=5000 > "$LOG_DIR/$pod.log" 2>&1 || true
done

# 其他服务日志
echo "  - 其他服务..."
for app in message-service conversation-service identity-service presence-service file-service; do
    for pod in $(kubectl get pods -n "$NAMESPACE" -l app="$app" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
        kubectl logs "$pod" -n "$NAMESPACE" --tail=1000 > "$LOG_DIR/$pod.log" 2>&1 || true
    done
done

# 10. Metrics 和应用指标
echo -e "${GREEN}>>> [10/12] 收集应用指标...${NC}"
METRICS_DIR="$RESULT_DIR/metrics"
mkdir -p "$METRICS_DIR"

# Delivery Service metrics
DELIVERY_POD=$(kubectl get pods -n "$NAMESPACE" -l app=delivery-service -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$DELIVERY_POD" ]; then
    echo "  - Delivery Service metrics..."
    kubectl exec "$DELIVERY_POD" -n "$NAMESPACE" -- wget -qO- http://localhost:8084/stats 2>/dev/null > "$METRICS_DIR/delivery-stats.json" || true
    kubectl exec "$DELIVERY_POD" -n "$NAMESPACE" -- wget -qO- http://localhost:8084/metrics 2>/dev/null > "$METRICS_DIR/delivery-metrics.txt" || true
fi

# API Gateway metrics
API_POD=$(kubectl get pods -n "$NAMESPACE" -l app=api-gateway -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$API_POD" ]; then
    echo "  - API Gateway metrics..."
    kubectl exec "$API_POD" -n "$NAMESPACE" -- wget -qO- http://localhost:8080/metrics 2>/dev/null > "$METRICS_DIR/api-gateway-metrics.txt" || true
fi

# 11. 事件和错误
echo -e "${GREEN}>>> [11/12] 收集事件和错误...${NC}"
{
    echo "=== 最近事件（按时间排序）==="
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' 2>/dev/null | tail -100 || true
    echo ""
    echo "=== 警告和错误事件 ==="
    kubectl get events -n "$NAMESPACE" --field-selector type!=Normal 2>/dev/null || true
} > "$RESULT_DIR/events.txt" 2>&1

# 从日志中提取错误
{
    echo "=== 日志中的错误（ERROR/FATAL 关键词）==="
    echo ""
    for log in "$LOG_DIR"/*.log; do
        if [ -f "$log" ]; then
            ERROR_COUNT=$(grep -ciE "error|fatal|panic|exception" "$log" 2>/dev/null || echo 0)
            if [ "$ERROR_COUNT" -gt 0 ]; then
                echo "--- $(basename "$log") (${ERROR_COUNT} 条) ---"
                grep -iE "error|fatal|panic|exception" "$log" 2>/dev/null | head -20 || true
                echo ""
            fi
        fi
    done
} > "$RESULT_DIR/errors.txt" 2>&1

# 12. 生成综合摘要
echo -e "${GREEN}>>> [12/12] 生成综合摘要...${NC}"
cat > "$RESULT_DIR/summary.txt" << EOF
=== 压测数据收集摘要 ===
收集时间: $(date)
命名空间: $NAMESPACE

--- 环境信息 ---
操作系统: $(uname -s) $(uname -r)
K8s 版本: $(kubectl version --short 2>/dev/null | grep Server | cut -d' ' -f3 || echo "未知")

--- 节点资源 ---
$(kubectl top nodes 2>/dev/null || echo "metrics-server 不可用")

--- Pod 统计 ---
总 Pod 数: $(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
Running: $(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c Running || echo 0)
Pending: $(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c Pending || echo 0)
Failed: $(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c Failed || echo 0)
Completed: $(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c Completed || echo 0)

--- 各服务 Pod 数 ---
API Gateway:      $(kubectl get pods -n "$NAMESPACE" -l app=api-gateway --no-headers 2>/dev/null | wc -l)
Delivery Service: $(kubectl get pods -n "$NAMESPACE" -l app=delivery-service --no-headers 2>/dev/null | wc -l)
wsbench:          $(kubectl get pods -n "$NAMESPACE" -l app=wsbench --no-headers 2>/dev/null | wc -l)

--- Pod 资源使用 TOP 5 (CPU) ---
$(kubectl top pods -n "$NAMESPACE" --no-headers 2>/dev/null | sort -k2 -rh | head -5 || echo "metrics-server 不可用")

--- Pod 资源使用 TOP 5 (Memory) ---
$(kubectl top pods -n "$NAMESPACE" --no-headers 2>/dev/null | sort -k3 -rh | head -5 || echo "metrics-server 不可用")

--- HPA 状态 ---
$(kubectl get hpa -n "$NAMESPACE" 2>/dev/null || echo "未配置 HPA")

--- 最近错误事件 ---
$(kubectl get events -n "$NAMESPACE" --field-selector type!=Normal 2>/dev/null | tail -10 || echo "无错误事件")

--- 收集的文件 ---
$(ls -lh "$RESULT_DIR" | tail -n +2)

EOF

# 添加 wsbench 结果汇总（如果有）
if ls "$LOG_DIR"/wsbench-*.log &>/dev/null; then
    {
        echo ""
        echo "=== wsbench 测试结果 ==="
        echo ""
        for log in "$LOG_DIR"/wsbench-*.log; do
            if [ -f "$log" ]; then
                echo "--- $(basename "$log") ---"
                # 提取 JSON 结果
                grep -o '{.*}' "$log" 2>/dev/null | tail -1 | python3 -m json.tool 2>/dev/null || \
                    grep -E "(success|failed|latency|messages)" "$log" | head -10 || true
                echo ""
            fi
        done
    } >> "$RESULT_DIR/summary.txt"
fi

echo ""
echo -e "${GREEN}=== 数据收集完成 ===${NC}"
echo ""
echo "结果目录: $RESULT_DIR"
echo "完成时间: $(date)"
echo ""
echo "主要文件:"
echo "  - summary.txt        综合摘要"
echo "  - environment.txt    环境信息"
echo "  - cluster-info.txt   集群信息"
echo "  - nodes.txt          节点信息"
echo "  - pods.txt/yaml      Pod 状态"
echo "  - resource-usage.txt 资源使用"
echo "  - hpa.txt            HPA 状态"
echo "  - events.txt         K8s 事件"
echo "  - errors.txt         错误汇总"
echo "  - logs/              所有日志"
echo "  - metrics/           应用指标"
echo "  - describe/          Pod 详细描述"
echo ""
echo -e "${YELLOW}查看摘要: cat $RESULT_DIR/summary.txt${NC}"
echo ""

