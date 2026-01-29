#!/bin/bash
# =============================================================================
# WSL2 系统调优脚本 - 高并发 WebSocket 压测优化
# 用法: sudo bash tune-wsl.sh
# =============================================================================

set -e

echo "=========================================="
echo "  WSL2 高并发压测系统调优"
echo "=========================================="
echo ""

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请使用 sudo 运行此脚本"
    exit 1
fi

echo ">>> [1/4] 调整文件描述符限制..."
cat >> /etc/security/limits.conf << 'EOF'
# 高并发压测优化 - 由 tune-wsl.sh 添加
* soft nofile 1000000
* hard nofile 1000000
* soft nproc 500000
* hard nproc 500000
root soft nofile 1000000
root hard nofile 1000000
EOF

# 立即生效
ulimit -n 1000000 2>/dev/null || true

echo ">>> [2/4] 调整内核网络参数..."
cat >> /etc/sysctl.conf << 'EOF'

# =============================================================================
# 高并发 WebSocket 压测优化 - 由 tune-wsl.sh 添加
# =============================================================================

# --- 连接队列 ---
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# --- 端口范围（关键！压测机需要大量临时端口）---
net.ipv4.ip_local_port_range = 1024 65535

# --- TIME_WAIT 优化（压测后快速释放端口）---
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_max_tw_buckets = 1000000

# --- TCP 内存（大量连接需要更多内存）---
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 1048576
net.core.wmem_default = 1048576
net.ipv4.tcp_rmem = 4096 1048576 16777216
net.ipv4.tcp_wmem = 4096 1048576 16777216
net.ipv4.tcp_mem = 786432 1048576 1572864

# --- TCP 连接优化 ---
net.ipv4.tcp_max_orphans = 262144
net.ipv4.tcp_syncookies = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_syn_retries = 2
net.ipv4.tcp_synack_retries = 2

# --- 文件描述符 ---
fs.file-max = 2000000
fs.nr_open = 2000000

# --- 连接追踪（如果存在）---
# net.netfilter.nf_conntrack_max = 2000000
EOF

# 应用配置
sysctl -p 2>/dev/null || true

echo ">>> [3/4] 配置 PAM limits..."
if ! grep -q "pam_limits.so" /etc/pam.d/common-session 2>/dev/null; then
    echo "session required pam_limits.so" >> /etc/pam.d/common-session
fi

echo ">>> [4/4] 验证配置..."
echo ""
echo "文件描述符限制: $(ulimit -n)"
echo "somaxconn: $(sysctl -n net.core.somaxconn)"
echo "端口范围: $(sysctl -n net.ipv4.ip_local_port_range)"
echo "tcp_tw_reuse: $(sysctl -n net.ipv4.tcp_tw_reuse)"
echo "file-max: $(sysctl -n fs.file-max)"
echo ""

echo "=========================================="
echo "  ✅ WSL2 调优完成!"
echo "=========================================="
