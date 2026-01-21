#!/bin/bash
# 安装 metrics-server 到 Docker Desktop Kubernetes
# 用于 kubectl top nodes/pods 命令
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
METRICS_SERVER_VERSION="v0.7.0"

echo "=== 安装 Metrics Server ${METRICS_SERVER_VERSION} ==="

# 检查是否已安装
if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    echo "Metrics Server 已安装，检查状态..."
    kubectl get deployment metrics-server -n kube-system
    echo ""
    echo "如需重新安装，请先运行: kubectl delete -f https://github.com/kubernetes-sigs/metrics-server/releases/download/${METRICS_SERVER_VERSION}/components.yaml"
    exit 0
fi

# 下载并修改 metrics-server 配置
# Docker Desktop 需要添加 --kubelet-insecure-tls 参数
TMP_FILE=$(mktemp)
curl -sL "https://github.com/kubernetes-sigs/metrics-server/releases/download/${METRICS_SERVER_VERSION}/components.yaml" > "$TMP_FILE"

# 添加 --kubelet-insecure-tls 参数（Docker Desktop 必需）
sed -i.bak 's/- --metric-resolution=15s/- --metric-resolution=15s\n        - --kubelet-insecure-tls/g' "$TMP_FILE"

echo "应用 Metrics Server 配置..."
kubectl apply -f "$TMP_FILE"

rm -f "$TMP_FILE" "${TMP_FILE}.bak"

echo ""
echo "等待 Metrics Server 就绪..."
kubectl rollout status deployment/metrics-server -n kube-system --timeout=120s

echo ""
echo "=== Metrics Server 安装完成 ==="
echo ""
echo "验证命令:"
echo "  kubectl top nodes"
echo "  kubectl top pods -n im"
echo ""
echo "注意：首次启动后需要等待 1-2 分钟才能获取到指标数据"
