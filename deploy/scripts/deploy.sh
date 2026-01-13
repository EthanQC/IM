#!/bin/bash
# IM 项目一键部署脚本
# 使用方式:
#   首次部署: ./deploy.sh
#   更新部署: ./deploy.sh update
#   CI/CD 调用: ./deploy.sh cicd

set -e

# 获取脚本所在目录（deploy目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_DIR="$(dirname "$DEPLOY_DIR")"

MODE="${1:-full}"
COMPOSE_FILE="docker-compose.prod.yml"

echo "=========================================="
echo "  IM 项目部署"
echo "  模式: $MODE"
echo "=========================================="

cd "$DEPLOY_DIR"

# 检查配置文件
check_config() {
    if [ ! -f "$DEPLOY_DIR/$COMPOSE_FILE" ]; then
        if [ -f "$DEPLOY_DIR/docker-compose.prod.yml.example" ]; then
            echo "未找到 $COMPOSE_FILE，从示例文件创建..."
            cp docker-compose.prod.yml.example $COMPOSE_FILE
            echo "请编辑 $COMPOSE_FILE 配置后重新运行此脚本"
            exit 1
        else
            echo "错误: 未找到 $COMPOSE_FILE 和示例文件"
            exit 1
        fi
    fi

    if [ ! -f "$DEPLOY_DIR/.env" ]; then
        if [ -f "$DEPLOY_DIR/.env.example" ]; then
            echo "未找到 .env，从示例文件创建..."
            cp .env.example .env
            echo "请编辑 .env 配置后重新运行此脚本"
            exit 1
        else
            echo "错误: 未找到 .env 和示例文件"
            exit 1
        fi
    fi
}

# 完整部署（首次或重新部署）
deploy_full() {
    echo "[1/5] 检查配置文件..."
    check_config

    echo "[2/5] 构建服务镜像..."
    docker compose -f $COMPOSE_FILE build

    echo "[3/5] 停止旧容器..."
    docker compose -f $COMPOSE_FILE down --remove-orphans || true

    echo "[4/5] 启动服务..."
    docker compose -f $COMPOSE_FILE up -d

    echo "[5/5] 等待服务启动..."
    sleep 30
}

# 更新部署（拉取代码后更新）
deploy_update() {
    echo "[1/4] 拉取最新代码..."
    cd "$PROJECT_DIR"
    git pull origin main

    echo "[2/4] 重新构建服务..."
    cd "$DEPLOY_DIR"
    docker compose -f $COMPOSE_FILE build

    echo "[3/4] 滚动更新服务..."
    docker compose -f $COMPOSE_FILE up -d --no-deps --build \
        api-gateway \
        identity-service \
        message-service \
        conversation-service \
        delivery-service \
        file-service \
        presence-service

    echo "[4/4] 清理旧镜像..."
    docker image prune -f
}

# CI/CD 部署（从 GitHub Actions 调用）
deploy_cicd() {
    echo "[1/3] 拉取最新代码..."
    cd "$PROJECT_DIR"
    git pull origin main

    echo "[2/3] 重新构建并启动服务..."
    cd "$DEPLOY_DIR"
    docker compose -f $COMPOSE_FILE up -d --build

    echo "[3/3] 清理旧镜像..."
    docker image prune -f
}

# 执行部署
case $MODE in
    "full")
        deploy_full
        ;;
    "update")
        deploy_update
        ;;
    "cicd")
        deploy_cicd
        ;;
    *)
        echo "未知模式: $MODE"
        echo "可用模式: full(默认), update, cicd"
        exit 1
        ;;
esac

# 检查服务状态
echo ""
echo "服务状态:"
docker compose -f $COMPOSE_FILE ps

# 检查 API Gateway 健康状态
echo ""
echo "检查 API Gateway..."
for i in {1..10}; do
    if curl -s -o /dev/null -w "%{http_code}" http://localhost/healthz 2>/dev/null | grep -q "200"; then
        echo "✅ API Gateway 运行正常"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "⚠️ API Gateway 启动检查超时，请手动检查日志"
    else
        echo "等待 API Gateway 启动... ($i/10)"
        sleep 5
    fi
done

echo ""
echo "=========================================="
echo "  部署完成！"
echo "=========================================="
echo ""
PUBLIC_IP=$(curl -s --connect-timeout 5 ifconfig.me 2>/dev/null || echo "获取失败")
echo "访问地址: http://$PUBLIC_IP"
echo "API 文档: http://$PUBLIC_IP/swagger"
echo ""
echo "常用命令:"
echo "  查看日志: docker compose -f $COMPOSE_FILE logs -f"
echo "  停止服务: docker compose -f $COMPOSE_FILE down"
echo "  重启服务: docker compose -f $COMPOSE_FILE restart"
