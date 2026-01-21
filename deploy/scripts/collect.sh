#!/bin/bash
# 收集压测数据脚本
# 用法: ./collect.sh [namespace] [result_dir]

set -e

NAMESPACE="${1:-im}"
RESULT_DIR="${2:-$(dirname "$0")/../../bench/results/$(date +%Y%m%d_%H%M%S)}"

mkdir -p "$RESULT_DIR"

echo "=== 收集压测数据 ==="
echo "命名空间: $NAMESPACE"
echo "结果目录: $RESULT_DIR"
echo ""

# 1. 集群信息
echo ">>> 收集集群信息..."
kubectl cluster-info > "$RESULT_DIR/cluster-info.txt" 2>&1 || true
kubectl get nodes -o wide > "$RESULT_DIR/nodes.txt" 2>&1 || true

# 2. Pod 状态
echo ">>> 收集 Pod 状态..."
kubectl get pods -n "$NAMESPACE" -o wide > "$RESULT_DIR/pods.txt" 2>&1 || true
kubectl get pods -n "$NAMESPACE" -o yaml > "$RESULT_DIR/pods.yaml" 2>&1 || true

# 3. Service 状态
echo ">>> 收集 Service 状态..."
kubectl get svc -n "$NAMESPACE" -o wide > "$RESULT_DIR/services.txt" 2>&1 || true

# 4. HPA 状态
echo ">>> 收集 HPA 状态..."
kubectl get hpa -n "$NAMESPACE" > "$RESULT_DIR/hpa.txt" 2>&1 || true

# 5. 资源使用（需要 metrics-server）
echo ">>> 收集资源使用..."
if kubectl top nodes &>/dev/null; then
    kubectl top nodes > "$RESULT_DIR/top-nodes.txt" 2>&1 || true
    kubectl top pods -n "$NAMESPACE" > "$RESULT_DIR/top-pods.txt" 2>&1 || true
else
    echo "metrics-server 未安装，跳过资源使用收集" > "$RESULT_DIR/top-nodes.txt"
fi

# 6. Pod 描述（用于排查问题）
echo ">>> 收集 Pod 描述..."
for pod in $(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}'); do
    kubectl describe pod "$pod" -n "$NAMESPACE" > "$RESULT_DIR/describe-$pod.txt" 2>&1 || true
done

# 7. 日志收集
echo ">>> 收集日志..."
LOG_DIR="$RESULT_DIR/logs"
mkdir -p "$LOG_DIR"

# API Gateway 日志
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=api-gateway -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=1000 > "$LOG_DIR/$pod.log" 2>&1 || true
done

# Delivery Service 日志
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=delivery-service -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=1000 > "$LOG_DIR/$pod.log" 2>&1 || true
done

# wsbench 日志
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=wsbench -o jsonpath='{.items[*].metadata.name}'); do
    kubectl logs "$pod" -n "$NAMESPACE" --tail=2000 > "$LOG_DIR/$pod.log" 2>&1 || true
done

# 8. Metrics 收集
echo ">>> 收集指标..."
METRICS_DIR="$RESULT_DIR/metrics"
mkdir -p "$METRICS_DIR"

# 尝试从 Delivery Service 获取 metrics
DELIVERY_POD=$(kubectl get pods -n "$NAMESPACE" -l app=delivery-service -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$DELIVERY_POD" ]; then
    kubectl exec "$DELIVERY_POD" -n "$NAMESPACE" -- wget -qO- http://localhost:8084/stats > "$METRICS_DIR/delivery-stats.json" 2>&1 || true
    kubectl exec "$DELIVERY_POD" -n "$NAMESPACE" -- wget -qO- http://localhost:8084/metrics > "$METRICS_DIR/delivery-metrics.txt" 2>&1 || true
fi

# 9. 事件
echo ">>> 收集事件..."
kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' > "$RESULT_DIR/events.txt" 2>&1 || true

# 10. 生成摘要
echo ">>> 生成摘要..."
cat > "$RESULT_DIR/summary.txt" << EOF
=== 压测数据收集摘要 ===
时间: $(date)
命名空间: $NAMESPACE

--- Pod 状态 ---
$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l) 个 Pod

--- 资源使用 ---
$(cat "$RESULT_DIR/top-pods.txt" 2>/dev/null || echo "未收集")

--- HPA 状态 ---
$(cat "$RESULT_DIR/hpa.txt" 2>/dev/null || echo "未收集")

EOF

echo ""
echo "=== 收集完成 ==="
echo "结果目录: $RESULT_DIR"
echo ""
ls -la "$RESULT_DIR"
