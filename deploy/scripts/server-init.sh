#!/bin/bash
# IM 项目云服务器初始化脚本
# 适用于 Ubuntu 22.04 / Debian 12

set -e

echo "=========================================="
echo "  IM 项目云服务器初始化脚本"
echo "=========================================="

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then
    echo "请使用 root 用户运行此脚本"
    exit 1
fi

# 更新系统
echo "[1/5] 更新系统包..."
apt-get update && apt-get upgrade -y

# 安装必要工具
echo "[2/5] 安装必要工具..."
apt-get install -y curl git

# 安装 Docker
echo "[3/5] 安装 Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "Docker 安装完成"
else
    echo "Docker 已安装"
fi

# 检查 Docker Compose
echo "[4/5] 检查 Docker Compose..."
if ! docker compose version &> /dev/null; then
    echo "Docker Compose 插件未安装，正在安装..."
    apt-get install -y docker-compose-plugin
fi
echo "Docker Compose 版本: $(docker compose version)"

# 配置 Docker
echo "[5/5] 配置 Docker..."
mkdir -p /etc/docker
cat > /etc/docker/daemon.json << 'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2"
}
EOF
systemctl daemon-reload
systemctl restart docker

# 配置 Swap（小内存服务器需要）
echo "配置 Swap..."
if [ ! -f /swapfile ]; then
    # 获取内存大小（MB）
    MEM_MB=$(free -m | awk '/^Mem:/{print $2}')
    # Swap 大小 = 内存大小（最大4G）
    SWAP_SIZE=$((MEM_MB > 4096 ? 4096 : MEM_MB))
    
    fallocate -l ${SWAP_SIZE}M /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo "Swap 配置完成: ${SWAP_SIZE}MB"
else
    echo "Swap 已存在"
fi

# 优化系统参数
echo "优化系统参数..."
cat >> /etc/sysctl.conf << 'EOF'
# IM 项目优化参数
vm.swappiness=10
net.core.somaxconn=65535
net.ipv4.tcp_max_syn_backlog=65535
net.ipv4.tcp_fin_timeout=30
net.ipv4.tcp_keepalive_time=300
net.ipv4.tcp_tw_reuse=1
EOF
sysctl -p 2>/dev/null || true

echo ""
echo "=========================================="
echo "  初始化完成！"
echo "=========================================="
echo ""
echo "后续步骤:"
echo "1. 克隆项目: git clone https://github.com/你的用户名/IM.git"
echo "2. 进入目录: cd IM/deploy"
echo "3. 复制配置: cp docker-compose.prod.yml.example docker-compose.prod.yml"
echo "4. 复制环境变量: cp .env.example .env"
echo "5. 编辑配置: vim .env"
echo "6. 运行部署: ./scripts/deploy.sh"

