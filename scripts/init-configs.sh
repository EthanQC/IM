#!/bin/bash
# ============================================================
# 配置文件初始化脚本
# 功能：从 .example 模板复制配置文件，自动填充默认值
# 使用：bash scripts/init-configs.sh
# ============================================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  IM 项目配置文件初始化${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# 默认配置值（与 docker-compose.dev.yml 保持一致）
MYSQL_PASSWORD="imdev"
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_DB="im_db"
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"
KAFKA_BROKER="127.0.0.1:29092"
JWT_SECRET="your-dev-jwt-secret-key-at-least-32-characters"

# 服务列表
SERVICES=(
    "api_gateway"
    "identity_service"
    "conversation_service"
    "message_service"
    "presence_service"
    "file_service"
    "delivery_service"
)

# 环境选择
echo -e "${YELLOW}选择要初始化的环境：${NC}"
echo "  1) dev  - 本地开发环境（默认）"
echo "  2) prod - 生产环境"
echo "  3) both - 两者都初始化"
read -p "请选择 [1/2/3]: " ENV_CHOICE

case $ENV_CHOICE in
    2) ENVS=("prod") ;;
    3) ENVS=("dev" "prod") ;;
    *) ENVS=("dev") ;;
esac

echo ""
echo -e "${CYAN}>>> 开始初始化配置文件...${NC}"
echo ""

CREATED=0
SKIPPED=0

for service in "${SERVICES[@]}"; do
    for env in "${ENVS[@]}"; do
        EXAMPLE_FILE="$PROJECT_ROOT/services/$service/configs/config.$env.yaml.example"
        TARGET_FILE="$PROJECT_ROOT/services/$service/configs/config.$env.yaml"
        
        if [ ! -f "$EXAMPLE_FILE" ]; then
            echo -e "${RED}[错误] 模板不存在: $EXAMPLE_FILE${NC}"
            continue
        fi
        
        if [ -f "$TARGET_FILE" ]; then
            echo -e "${YELLOW}[跳过] 已存在: services/$service/configs/config.$env.yaml${NC}"
            ((SKIPPED++))
            continue
        fi
        
        # 复制并替换默认值
        cp "$EXAMPLE_FILE" "$TARGET_FILE"
        
        # 替换 MySQL 密码（从 your_password 改为实际密码）
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS
            sed -i '' "s/your_password/$MYSQL_PASSWORD/g" "$TARGET_FILE"
        else
            # Linux
            sed -i "s/your_password/$MYSQL_PASSWORD/g" "$TARGET_FILE"
        fi
        
        echo -e "${GREEN}[创建] services/$service/configs/config.$env.yaml${NC}"
        ((CREATED++))
    done
done

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${GREEN}初始化完成！${NC}"
echo -e "  创建: ${GREEN}$CREATED${NC} 个文件"
echo -e "  跳过: ${YELLOW}$SKIPPED${NC} 个文件（已存在）"
echo -e "${CYAN}========================================${NC}"
echo ""

# 验证
echo -e "${CYAN}>>> 验证配置文件...${NC}"
echo ""

MISSING=0
for service in "${SERVICES[@]}"; do
    DEV_FILE="$PROJECT_ROOT/services/$service/configs/config.dev.yaml"
    if [ -f "$DEV_FILE" ]; then
        echo -e "${GREEN}✓${NC} $service"
    else
        echo -e "${RED}✗${NC} $service - 缺少 config.dev.yaml"
        ((MISSING++))
    fi
done

echo ""

if [ $MISSING -eq 0 ]; then
    echo -e "${GREEN}所有服务配置文件就绪！${NC}"
    echo ""
    echo -e "${CYAN}下一步：${NC}"
    echo "  1. 启动依赖服务:  make docker-deps-up"
    echo "  2. 初始化数据库:  mysql -h 127.0.0.1 -u root -p$MYSQL_PASSWORD < deploy/sql/schema.sql"
    echo "  3. 启动微服务:    参考 README.md"
else
    echo -e "${RED}有 $MISSING 个服务缺少配置文件，请检查。${NC}"
    exit 1
fi
