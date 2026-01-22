#!/bin/bash
# 安装并配置 metrics-server 到 Docker Desktop Kubernetes
# 适配 Docker Desktop 环境（需要 --kubelet-insecure-tls 和 --kubelet-preferred-address-types）
# 用于 kubectl top nodes/pods 命令

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
METRICS_SERVER_VERSION="v0.7.2"

echo "=== 安装 Metrics Server ${METRICS_SERVER_VERSION} for Docker Desktop ==="
echo ""

# 检查是否已安装
if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    echo "[INFO] Metrics Server 已存在，检查状态..."
    kubectl get deployment metrics-server -n kube-system
    
    REPLICAS=$(kubectl get deployment metrics-server -n kube-system -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    if [ "$REPLICAS" -gt 0 ]; then
        echo ""
        echo "[SUCCESS] Metrics Server 运行正常"
        echo ""
        echo "验证命令:"
        echo "  kubectl top nodes"
        echo "  kubectl top pods -n im"
        exit 0
    else
        echo ""
        echo "[WARN] Metrics Server 未就绪，将重新部署"
        kubectl delete deployment metrics-server -n kube-system --ignore-not-found=true
        sleep 5
    fi
fi

# 下载并修改 metrics-server 配置
# Docker Desktop 需要特殊配置
TMP_FILE=$(mktemp)
echo "[INFO] 下载 Metrics Server 配置..."
curl -sL "https://github.com/kubernetes-sigs/metrics-server/releases/download/${METRICS_SERVER_VERSION}/components.yaml" > "$TMP_FILE"

# 为 Docker Desktop 添加必需参数
echo "[INFO] 适配 Docker Desktop 环境..."
# macOS sed 需要加空字符串参数
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' '/- --metric-resolution=15s/a\
        - --kubelet-insecure-tls\
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
' "$TMP_FILE"
else
    sed -i '/- --metric-resolution=15s/a\        - --kubelet-insecure-tls\n        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname' "$TMP_FILE"
fi

echo "[INFO] 应用 Metrics Server 配置..."
kubectl apply -f "$TMP_FILE"

rm -f "$TMP_FILE"

echo ""
echo "[INFO] 等待 Metrics Server 就绪（超时 180 秒）..."
if kubectl rollout status deployment/metrics-server -n kube-system --timeout=180s; then
    echo ""
    echo "[SUCCESS] Metrics Server 已就绪"
else
    echo ""
    echo "[ERROR] Metrics Server 部署超时，开始排障..."
    echo ""
    echo "--- Deployment 状态 ---"
    kubectl get deployment metrics-server -n kube-system
    echo ""
    echo "--- Pod 状态 ---"
    kubectl get pods -n kube-system -l k8s-app=metrics-server
    echo ""
    echo "--- Pod 描述 ---"
    kubectl describe pod -n kube-system -l k8s-app=metrics-server
    echo ""
    echo "--- Pod 日志 ---"
    kubectl logs -n kube-system -l k8s-app=metrics-server --tail=50 || true
    echo ""
    echo "--- Events ---"
    kubectl get events -n kube-system --sort-by='.lastTimestamp' | grep metrics-server || true
    echo ""
    echo "[FAIL] 请检查上述错误信息并排障"
    exit 1
fi

# 等待指标数据可用（最多等待 2 分钟）
echo ""
echo "[INFO] 等待指标数据可用（最多 120 秒）..."
TIMEOUT=120
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    if kubectl top nodes &>/dev/null; then
        echo ""
        echo "[SUCCESS] Metrics API 可用"
        echo ""
        echo "--- 节点资源使用 ---"
        kubectl top nodes
        echo ""
        break
    fi
    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo ""
    echo "[WARN] 指标数据仍未可用，但 Metrics Server 已部署"
    echo "可能原因："
    echo "  1. 节点压力过大，kubelet 响应慢"
    echo "  2. 需要更长时间采集数据"
    echo ""
    echo "请尝试："
    echo "  - 等待 1-2 分钟后运行: kubectl top nodes"
    echo "  - 检查日志: kubectl logs -n kube-system -l k8s-app=metrics-server"
    exit 1
fi

echo ""
echo "=== Metrics Server 安装完成 ==="
echo ""
echo "可用命令:"
echo "  kubectl top nodes                    # 查看节点资源使用"
echo "  kubectl top pods -n im               # 查看 im 命名空间 Pod 资源"
echo "  kubectl top pods -A                   # 查看所有 Pod 资源"
echo ""
