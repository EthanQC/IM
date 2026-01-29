#!/bin/bash
# =============================================================================
# 压测客户端系统调优脚本
# 用于在压测机器上运行，确保能够建立50k+连接
# 用法: sudo bash tune-bench-client.sh
# =============================================================================

set -e

echo "=========================================="
echo "  压测客户端系统调优"
echo "=========================================="
echo ""

# 检测操作系统
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
elif [[ -f /etc/os-release ]]; then
    OS="linux"
else
    echo "❌ 不支持的操作系统"
    exit 1
fi

echo "检测到操作系统: $OS"
echo ""

# 检查是否为 root（Linux）或有sudo权限（macOS）
if [ "$EUID" -ne 0 ] && [ "$OS" == "linux" ]; then
    echo "❌ 请使用 sudo 运行此脚本"
    exit 1
fi

if [ "$OS" == "macos" ]; then
    echo ">>> [1/4] 调整 macOS 内核参数..."
    
    # 文件描述符（支持 100k+ 连接）
    sudo sysctl -w kern.maxfiles=1000000 2>/dev/null || true
    sudo sysctl -w kern.maxfilesperproc=500000 2>/dev/null || true
    
    # TCP 连接队列
    sudo sysctl -w kern.ipc.somaxconn=8192 2>/dev/null || true
    
    # 扩大本地端口范围（关键！每个连接需要一个本地端口）
    sudo sysctl -w net.inet.ip.portrange.first=1024 2>/dev/null || true
    sudo sysctl -w net.inet.ip.portrange.last=65535 2>/dev/null || true
    
    # TCP 参数优化
    sudo sysctl -w net.inet.tcp.msl=15000 2>/dev/null || true
    
    # Socket 缓冲区
    sudo sysctl -w kern.ipc.maxsockbuf=16777216 2>/dev/null || true
    sudo sysctl -w net.inet.tcp.sendspace=1048576 2>/dev/null || true
    sudo sysctl -w net.inet.tcp.recvspace=1048576 2>/dev/null || true
    
    echo ">>> [2/4] 调整 launchctl 限制..."
    sudo launchctl limit maxfiles 500000 1000000 2>/dev/null || true
    
    echo ">>> [3/4] 验证配置..."
    echo ""
    echo "=== 当前系统配置 ==="
    echo "maxfiles: $(sysctl -n kern.maxfiles 2>/dev/null || echo 'N/A')"
    echo "maxfilesperproc: $(sysctl -n kern.maxfilesperproc 2>/dev/null || echo 'N/A')"
    echo "somaxconn: $(sysctl -n kern.ipc.somaxconn 2>/dev/null || echo 'N/A')"
    echo "端口范围: $(sysctl -n net.inet.ip.portrange.first 2>/dev/null || echo 'N/A') - $(sysctl -n net.inet.ip.portrange.last 2>/dev/null || echo 'N/A')"
    
    # 计算可用端口数
    PORT_FIRST=$(sysctl -n net.inet.ip.portrange.first 2>/dev/null || echo 1024)
    PORT_LAST=$(sysctl -n net.inet.ip.portrange.last 2>/dev/null || echo 65535)
    AVAIL_PORTS=$((PORT_LAST - PORT_FIRST))
    echo "可用端口数: $AVAIL_PORTS"
    echo ""

else
    # Linux 调优
    echo ">>> [1/4] 调整 Linux 内核参数..."
    
    # 文件描述符
    sysctl -w fs.file-max=2000000
    sysctl -w fs.nr_open=2000000
    
    # TCP 连接优化
    sysctl -w net.core.somaxconn=65535
    sysctl -w net.ipv4.tcp_max_syn_backlog=65535
    
    # 本地端口范围（关键！）
    sysctl -w net.ipv4.ip_local_port_range="1024 65535"
    
    # TIME_WAIT 优化
    sysctl -w net.ipv4.tcp_tw_reuse=1
    sysctl -w net.ipv4.tcp_fin_timeout=15
    
    # TCP 内存优化
    sysctl -w net.ipv4.tcp_mem="786432 1048576 1572864"
    sysctl -w net.ipv4.tcp_rmem="4096 87380 16777216"
    sysctl -w net.ipv4.tcp_wmem="4096 65536 16777216"
    
    # Socket 缓冲区
    sysctl -w net.core.rmem_max=16777216
    sysctl -w net.core.wmem_max=16777216
    sysctl -w net.core.rmem_default=262144
    sysctl -w net.core.wmem_default=262144
    
    # 网络队列
    sysctl -w net.core.netdev_max_backlog=65535
    sysctl -w net.core.optmem_max=25165824
    
    echo ">>> [2/4] 调整文件描述符限制..."
    
    # 持久化配置
    cat > /etc/security/limits.d/99-bench.conf << EOF
* soft nofile 500000
* hard nofile 1000000
* soft nproc 500000
* hard nproc 1000000
root soft nofile 500000
root hard nofile 1000000
EOF
    
    echo ">>> [3/4] 验证配置..."
    echo ""
    echo "=== 当前系统配置 ==="
    echo "fs.file-max: $(sysctl -n fs.file-max)"
    echo "somaxconn: $(sysctl -n net.core.somaxconn)"
    echo "端口范围: $(sysctl -n net.ipv4.ip_local_port_range)"
    echo ""
fi

echo ">>> [4/4] 检查当前 shell 限制..."
echo "当前 ulimit -n: $(ulimit -n)"

# 尝试设置 ulimit
ulimit -n 500000 2>/dev/null || echo "⚠️  无法在当前 shell 设置 ulimit，请重新登录或使用新终端"

echo ""
echo "=========================================="
echo "  ✅ 系统调优完成!"
echo "=========================================="
echo ""
echo "⚠️  重要提示:"
echo "  1. macOS: 这些设置重启后会失效"
echo "  2. Linux: limits.conf 需要重新登录生效"
echo "  3. 运行压测前请确认: ulimit -n 返回至少 100000"
echo "  4. 每台机器最多可建立约 64000 个连接（受端口限制）"
echo "  5. 如需超过 64k 连接，需要配置多个本地 IP 地址"
echo ""
echo "运行压测命令示例:"
echo "  ulimit -n 500000 && ./wsbench --target=ws://服务器IP:8084/ws --conns=50000 --ramp=5m --duration=10m"
echo ""
