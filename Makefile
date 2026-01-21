# IM 项目 Makefile
# 支持 K8s 部署、压测、数据收集

.PHONY: help build build-gateway build-delivery build-wsbench \
        k8s-up k8s-down k8s-status k8s-logs \
        bench-ws-50k bench-ws-msg bench-collect bench-stop \
        docker-deps-up docker-deps-down \
        install-metrics-server clean

# 默认目标
.DEFAULT_GOAL := help

# 项目路径
PROJECT_ROOT := $(shell pwd)
DEPLOY_DIR := $(PROJECT_ROOT)/deploy
K8S_OVERLAY := $(DEPLOY_DIR)/k8s/overlays/docker-desktop
BENCH_DIR := $(PROJECT_ROOT)/bench
RESULTS_DIR := $(BENCH_DIR)/results

# K8s 命名空间
NAMESPACE := im

# 颜色输出
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m

help: ## 显示帮助信息
	@echo "$(CYAN)IM 项目 Makefile$(NC)"
	@echo ""
	@echo "$(GREEN)构建命令:$(NC)"
	@grep -E '^build[a-zA-Z_-]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)K8s 部署命令:$(NC)"
	@grep -E '^k8s-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)压测命令:$(NC)"
	@grep -E '^bench-[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)其他命令:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -vE '^(build|k8s-|bench-)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[0m %s\n", $$1, $$2}'

# ============== 构建命令 ==============

build: build-gateway build-delivery build-wsbench ## 构建所有镜像

build-gateway: ## 构建 API Gateway 镜像
	@echo "$(CYAN)>>> 构建 API Gateway 镜像...$(NC)"
	docker build -t im/api-gateway:latest -f $(PROJECT_ROOT)/services/api_gateway/Dockerfile $(PROJECT_ROOT)

build-delivery: ## 构建 Delivery Service 镜像
	@echo "$(CYAN)>>> 构建 Delivery Service 镜像...$(NC)"
	docker build -t im/delivery-service:latest -f $(PROJECT_ROOT)/services/delivery_service/Dockerfile $(PROJECT_ROOT)

build-wsbench: ## 构建 wsbench 压测工具镜像
	@echo "$(CYAN)>>> 构建 wsbench 镜像...$(NC)"
	docker build -t im/wsbench:latest $(BENCH_DIR)/wsbench

# ============== K8s 部署命令 ==============

k8s-up: ## 部署到 Docker Desktop K8s
	@echo "$(CYAN)>>> 部署到 Kubernetes...$(NC)"
	kubectl apply -k $(K8S_OVERLAY)
	@echo ""
	@echo "$(GREEN)>>> 等待 Pod 就绪...$(NC)"
	kubectl wait --for=condition=ready pod -l app=api-gateway -n $(NAMESPACE) --timeout=120s || true
	kubectl wait --for=condition=ready pod -l app=delivery-service -n $(NAMESPACE) --timeout=120s || true
	@echo ""
	@echo "$(GREEN)>>> 部署完成!$(NC)"
	@$(MAKE) k8s-status

k8s-down: ## 清理 K8s 资源
	@echo "$(YELLOW)>>> 清理 Kubernetes 资源...$(NC)"
	kubectl delete -k $(K8S_OVERLAY) --ignore-not-found=true
	@echo "$(GREEN)>>> 清理完成$(NC)"

k8s-status: ## 查看 K8s 状态
	@echo "$(CYAN)>>> K8s 资源状态$(NC)"
	@echo ""
	@echo "--- Pods ---"
	@kubectl get pods -n $(NAMESPACE) -o wide 2>/dev/null || echo "命名空间 $(NAMESPACE) 不存在"
	@echo ""
	@echo "--- Services ---"
	@kubectl get svc -n $(NAMESPACE) 2>/dev/null || true
	@echo ""
	@echo "--- HPA ---"
	@kubectl get hpa -n $(NAMESPACE) 2>/dev/null || true
	@echo ""
	@echo "$(GREEN)访问地址:$(NC)"
	@echo "  API Gateway:      http://localhost:30080"
	@echo "  Delivery Service: ws://localhost:30084/ws"

k8s-logs: ## 查看 K8s 日志 (用法: make k8s-logs APP=delivery-service)
	@if [ -z "$(APP)" ]; then \
		echo "用法: make k8s-logs APP=<app-name>"; \
		echo "可用: api-gateway, delivery-service, wsbench"; \
	else \
		kubectl logs -f -l app=$(APP) -n $(NAMESPACE) --tail=100; \
	fi

k8s-restart: ## 重启 K8s 部署 (用法: make k8s-restart APP=delivery-service)
	@if [ -z "$(APP)" ]; then \
		echo "$(CYAN)>>> 重启所有部署...$(NC)"; \
		kubectl rollout restart deployment -n $(NAMESPACE); \
	else \
		echo "$(CYAN)>>> 重启 $(APP)...$(NC)"; \
		kubectl rollout restart deployment $(APP) -n $(NAMESPACE); \
	fi

# ============== 压测命令 ==============

bench-ws-50k: ## 启动 50k WebSocket connect-only 压测
	@echo "$(CYAN)>>> 启动 50k WebSocket 压测$(NC)"
	@echo "配置: 20 个 Pod × 2500 连接 = 50000 并发"
	@echo ""
	@# 先确保 wsbench 镜像存在
	@docker images im/wsbench:latest -q | grep -q . || $(MAKE) build-wsbench
	@# 设置环境变量并扩容
	kubectl set env deployment/wsbench -n $(NAMESPACE) \
		BENCH_MODE=connect-only \
		CONNS_PER_POD=2500 \
		DURATION=10m \
		RAMP_DURATION=2m
	kubectl scale deployment/wsbench -n $(NAMESPACE) --replicas=20
	@echo ""
	@echo "$(GREEN)>>> 压测已启动，使用以下命令查看状态:$(NC)"
	@echo "  make k8s-status"
	@echo "  make k8s-logs APP=wsbench"
	@echo "  make bench-collect"

bench-ws-10k: ## 启动 10k WebSocket 压测（轻量级测试）
	@echo "$(CYAN)>>> 启动 10k WebSocket 压测$(NC)"
	@echo "配置: 10 个 Pod × 1000 连接 = 10000 并发"
	kubectl set env deployment/wsbench -n $(NAMESPACE) \
		BENCH_MODE=connect-only \
		CONNS_PER_POD=1000 \
		DURATION=5m \
		RAMP_DURATION=1m
	kubectl scale deployment/wsbench -n $(NAMESPACE) --replicas=10
	@echo "$(GREEN)>>> 压测已启动$(NC)"

bench-ws-msg: ## 启动消息压测
	@echo "$(CYAN)>>> 启动消息压测$(NC)"
	kubectl set env deployment/wsbench -n $(NAMESPACE) \
		BENCH_MODE=messaging \
		CONNS_PER_POD=1000 \
		DURATION=5m \
		RAMP_DURATION=1m
	kubectl scale deployment/wsbench -n $(NAMESPACE) --replicas=10
	@echo "$(GREEN)>>> 消息压测已启动$(NC)"

bench-stop: ## 停止压测
	@echo "$(YELLOW)>>> 停止压测...$(NC)"
	kubectl scale deployment/wsbench -n $(NAMESPACE) --replicas=0
	@echo "$(GREEN)>>> 压测已停止$(NC)"

bench-collect: ## 收集压测数据
	@echo "$(CYAN)>>> 收集压测数据...$(NC)"
	@mkdir -p $(RESULTS_DIR)
	@chmod +x $(DEPLOY_DIR)/scripts/collect.sh
	@$(DEPLOY_DIR)/scripts/collect.sh $(NAMESPACE) $(RESULTS_DIR)/$$(date +%Y%m%d_%H%M%S)

bench-local: ## 本地运行压测（直接连接 NodePort）
	@echo "$(CYAN)>>> 本地运行压测$(NC)"
	cd $(BENCH_DIR)/wsbench && go run . \
		--target=ws://localhost:30084/ws \
		--conns=1000 \
		--duration=2m \
		--ramp=30s

# ============== 依赖管理 ==============

docker-deps-up: ## 启动宿主机依赖（MySQL/Redis/Kafka/MinIO）
	@echo "$(CYAN)>>> 启动宿主机依赖...$(NC)"
	docker-compose -f $(DEPLOY_DIR)/docker-compose.dev.yml up -d
	@echo "$(GREEN)>>> 依赖服务已启动$(NC)"

docker-deps-down: ## 停止宿主机依赖
	@echo "$(YELLOW)>>> 停止宿主机依赖...$(NC)"
	docker-compose -f $(DEPLOY_DIR)/docker-compose.dev.yml down
	@echo "$(GREEN)>>> 依赖服务已停止$(NC)"

docker-deps-status: ## 查看宿主机依赖状态
	@docker-compose -f $(DEPLOY_DIR)/docker-compose.dev.yml ps

# ============== 工具安装 ==============

install-metrics-server: ## 安装 metrics-server（用于 kubectl top）
	@echo "$(CYAN)>>> 安装 metrics-server...$(NC)"
	@chmod +x $(DEPLOY_DIR)/scripts/install-metrics-server.sh
	@$(DEPLOY_DIR)/scripts/install-metrics-server.sh

# ============== 清理 ==============

clean: ## 清理构建产物
	@echo "$(YELLOW)>>> 清理...$(NC)"
	rm -rf $(RESULTS_DIR)/*
	docker rmi im/api-gateway:latest im/delivery-service:latest im/wsbench:latest 2>/dev/null || true
	@echo "$(GREEN)>>> 清理完成$(NC)"

# ============== 完整流程 ==============

full-deploy: docker-deps-up build k8s-up ## 完整部署流程（启动依赖 + 构建 + 部署）
	@echo "$(GREEN)>>> 完整部署完成!$(NC)"

full-bench: bench-ws-10k ## 完整压测流程（10k 连接）
	@echo "等待 30 秒让压测稳定..."
	@sleep 30
	@$(MAKE) bench-collect
	@echo "等待压测完成..."
	@sleep 270
	@$(MAKE) bench-collect
	@$(MAKE) bench-stop
