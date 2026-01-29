#!/bin/bash
# =============================================================================
# WebSocket 服务端系统调优脚本（用于支持 100k+ 并发连接）
# 用法: sudo bash tune-server.sh
# =============================================================================

set -e

echo "=========================================="
echo "  WebSocket 服务端高并发调优"
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
echo "目标: 支持 100,000+ 并发 WebSocket 连接"
echo ""

if [ "$OS" == "macos" ]; then
    echo ">>> 调整 macOS 内核参数..."
    
    # 检查是否有 sudo 权限
    if [ "$EUID" -ne 0 ]; then
        echo "需要 sudo 权限，请输入密码..."
    fi
    
    # 文件描述符（服务端需要更大）
    sudo sysctl -w kern.maxfiles=2000000
    sudo sysctl -w kern.maxfilesperproc=1000000
    
    # TCP 连接队列（关键！处理大量并发连接请求）
    sudo sysctl -w kern.ipc.somaxconn=65535
    
    # 端口范围
    sudo sysctl -w net.inet.ip.portrange.first=1024
    sudo sysctl -w net.inet.ip.portrange.last=65535
    
    # TCP 优化
    sudo sysctl -w net.inet.tcp.msl=15000
    
    # Socket 缓冲区（macOS 有上限，使用安全值）
    # 注意：macOS 的 maxsockbuf 有系统限制，不能设置太大
    sudo sysctl -w kern.ipc.maxsockbuf=16777216 2>/dev/null || echo "maxsockbuf 保持默认值"
    sudo sysctl -w net.inet.tcp.sendspace=1048576 2>/dev/null || echo "sendspace 保持默认值"
    sudo sysctl -w net.inet.tcp.recvspace=1048576 2>/dev/null || echo "recvspace 保持默认值"
    
    # launchctl 限制
    sudo launchctl limit maxfiles 1000000 2000000
    
    echo ""
    echo "=== 当前配置 ==="
    echo "maxfiles: $(sysctl -n kern.maxfiles)"
    echo "maxfilesperproc: $(sysctl -n kern.maxfilesperproc)"
    echo "somaxconn: $(sysctl -n kern.ipc.somaxconn)"
    echo "maxsockbuf: $(sysctl -n kern.ipc.maxsockbuf)"
    echo "launchctl maxfiles: $(launchctl limit maxfiles 2>/dev/null | awk '{print $2, $3}')"
    
else
    echo ">>> 调整 Linux 内核参数..."
    
    # 检查 root 权限
    if [ "$EUID" -ne 0 ]; then
        echo "❌ 请使用 sudo 运行此脚本"
        exit 1
    fi
    
    # 文件描述符
    sysctl -w fs.file-max=2000000
    sysctl -w fs.nr_open=2000000
    
    # TCP 连接队列（关键！）
    sysctl -w net.core.somaxconn=65535
    sysctl -w net.ipv4.tcp_max_syn_backlog=65535
    
    # 本地端口范围
    sysctl -w net.ipv4.ip_local_port_range="1024 65535"
    
    # TIME_WAIT 状态优化
    sysctl -w net.ipv4.tcp_tw_reuse=1
    sysctl -w net.ipv4.tcp_fin_timeout=15
    sysctl -w net.ipv4.tcp_max_tw_buckets=1440000
    
    # TCP keepalive 优化
    sysctl -w net.ipv4.tcp_keepalive_time=300
    sysctl -w net.ipv4.tcp_keepalive_probes=3
    sysctl -w net.ipv4.tcp_keepalive_intvl=30
    
    # TCP 内存（单位: 页面，4KB/页）
    # min: 3GB, pressure: 4GB, max: 6GB
    sysctl -w net.ipv4.tcp_mem="786432 1048576 1572864"
    sysctl -w net.ipv4.tcp_rmem="4096 87380 16777216"
    sysctl -w net.ipv4.tcp_wmem="4096 65536 16777216"
    
    # Socket 缓冲区
    sysctl -w net.core.rmem_max=33554432
    sysctl -w net.core.wmem_max=33554432
    sysctl -w net.core.rmem_default=262144
    sysctl -w net.core.wmem_default=262144
    
    # 网络队列
    sysctl -w net.core.netdev_max_backlog=65535
    sysctl -w net.core.optmem_max=25165824
    
    # 文件描述符限制
    cat > /etc/security/limits.d/99-websocket.conf << EOF
* soft nofile 1000000
* hard nofile 2000000
* soft nproc 500000
* hard nproc 1000000
root soft nofile 1000000
root hard nofile 2000000
EOF
    
    # 持久化 sysctl 配置
    cat > /etc/sysctl.d/99-websocket.conf << EOF
# WebSocket 高并发优化
fs.file-max = 2000000
fs.nr_open = 2000000
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_max_tw_buckets = 1440000
net.ipv4.tcp_keepalive_time = 300
net.ipv4.tcp_keepalive_probes = 3
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_mem = 786432 1048576 1572864
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.core.rmem_max = 33554432
net.core.wmem_max = 33554432
net.core.rmem_default = 262144
net.core.wmem_default = 262144
net.core.netdev_max_backlog = 65535
net.core.optmem_max = 25165824
EOF
    
    echo ""
    echo "=== 当前配置 ==="
    echo "fs.file-max: $(sysctl -n fs.file-max)"
    echo "somaxconn: $(sysctl -n net.core.somaxconn)"
    echo "tcp_max_syn_backlog: $(sysctl -n net.ipv4.tcp_max_syn_backlog)"
    echo "端口范围: $(sysctl -n net.ipv4.ip_local_port_range)"
fi

echo ""
echo "=========================================="
echo "  ✅ 服务端调优完成!"
echo "=========================================="
echo ""
echo "⚠️  后续步骤:"
echo "  1. 确保运行服务前设置: ulimit -n 1000000"
echo "  2. Linux 系统需要重新登录使 limits.conf 生效"
echo "  3. 监控命令:"
echo "     - 查看连接数: netstat -an | grep ESTABLISHED | wc -l"
echo "     - 查看文件描述符: ls /proc/\$(pgrep delivery)/fd | wc -l"
echo "     - 查看 goroutine: curl localhost:8084/metrics | grep go_goroutines"
echo ""
