#!/bin/bash
# =============================================================================
# macOS 系统调优脚本 - 高并发 WebSocket 服务端优化
# 用法: sudo bash tune-macos.sh
# =============================================================================

set -e

echo "=========================================="
echo "  macOS 高并发服务端系统调优"
echo "=========================================="
echo ""

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请使用 sudo 运行此脚本"
    exit 1
fi

echo ">>> [1/3] 调整内核参数..."

# 文件描述符
sysctl -w kern.maxfiles=2000000
sysctl -w kern.maxfilesperproc=1000000

# 网络参数
sysctl -w net.inet.ip.portrange.first=1024
sysctl -w net.inet.ip.portrange.last=65535
sysctl -w net.inet.tcp.msl=15000

# Socket 缓冲区
sysctl -w kern.ipc.maxsockbuf=16777216
sysctl -w net.inet.tcp.sendspace=1048576
sysctl -w net.inet.tcp.recvspace=1048576

echo ">>> [2/3] 调整 launchctl 限制..."
launchctl limit maxfiles 1000000 2000000

echo ">>> [3/3] 验证配置..."
echo ""
echo "maxfiles: $(sysctl -n kern.maxfiles)"
echo "maxfilesperproc: $(sysctl -n kern.maxfilesperproc)"
echo "端口范围: $(sysctl -n net.inet.ip.portrange.first) - $(sysctl -n net.inet.ip.portrange.last)"
echo "launchctl maxfiles: $(launchctl limit maxfiles)"
echo ""

echo "=========================================="
echo "  ✅ macOS 调优完成!"
echo "=========================================="
echo ""
echo "⚠️  重要提示:"
echo "  1. 这些设置重启后会失效"
echo "  2. 如需永久生效，可创建 /etc/sysctl.conf 文件"
echo "  3. 建议在启动服务前执行: ulimit -n 1000000"
echo ""
