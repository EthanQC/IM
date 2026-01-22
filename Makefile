# IM 项目 Makefile
# 支持 K8s 部署、压测、数据收集

.PHONY: help build build-gateway build-delivery build-wsbench \
        k8s-up k8s-down k8s-status k8s-logs k8s-restart \
        bench-ws-1k bench-ws-5k bench-ws-10k bench-ws-50k bench-msg-throughput bench-collect bench-stop bench-local \
        docker-deps-up docker-deps-down docker-deps-status \
        install-metrics-server verify-metrics clean

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
RED := \033[0;31m
NC := \033[0m

help: ## 显示帮助信息
	@echo "$(CYAN)IM 项目 Makefile - 压测与部署自动化$(NC)"
	@echo ""
	@echo "$(GREEN)构建命令:$(NC)"
	@grep -E '^build[a-zA-Z_-]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)K8s 部署命令:$(NC)"
	@grep -E '^k8s-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)压测命令:$(NC)"
	@grep -E '^bench-[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)依赖管理:$(NC)"
	@grep -E '^docker-deps-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)其他命令:$(NC)"
	@grep -E '^(install|verify|clean)[a-zA-Z_-]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)快速开始:$(NC)"
	@echo "  1. make docker-deps-up         # 启动依赖服务"
	@echo "  2. make install-metrics-server # 安装监控"
	@echo "  3. make build                  # 构建镜像"
	@echo "  4. make k8s-up                 # 部署到 K8s"
	@echo "  5. make bench-ws-10k           # 运行 10k 压测"
	@echo "  6. make bench-collect          # 收集测试数据"


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

k8s-up: ## 部署到 Docker Desktop K8s（应用 docker-desktop overlay）
	@echo "$(CYAN)>>> 部署到 Docker Desktop Kubernetes...$(NC)"
	@echo "使用配置: $(K8S_OVERLAY)"
	@echo ""
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

k8s-status: ## 查看 K8s 状态（Pods/Services/HPA）
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
		echo "$(RED)用法: make k8s-logs APP=<app-name>$(NC)"; \
		echo "可用: api-gateway, delivery-service, wsbench"; \
		exit 1; \
	else \
		echo "$(CYAN)>>> 查看 $(APP) 日志...$(NC)"; \
		kubectl logs -f -l app=$(APP) -n $(NAMESPACE) --tail=100; \
	fi

k8s-restart: ## 重启 K8s 部署 (用法: make k8s-restart APP=delivery-service)
	@if [ -z "$(APP)" ]; then \
		echo "$(CYAN)>>> 重启所有部署...$(NC)"; \
		kubectl rollout restart deployment -n $(NAMESPACE); \
	else \
		echo "$(CYAN)>>> 重启 $(APP)...$(NC)"; \
		kubectl rollout restart deployment $(APP) -n $(NAMESPACE); \
		kubectl rollout status deployment $(APP) -n $(NAMESPACE); \
	fi

# ============== 压测命令 ==============

bench-ws-1k: ## WebSocket 1k 连接压测（快速验证）
	@echo "$(CYAN)>>> 启动 1k WebSocket 压测（快速验证）$(NC)"
	@echo "配置: 2 Pod × 500 连接 = 1000 并发"
	@echo "持续: 3 分钟, 爬坡: 30 秒"
	@echo ""
	@chmod +x $(BENCH_DIR)/scripts/bench-ws.sh
	@$(BENCH_DIR)/scripts/bench-ws.sh 1000 2 500 3m 30s

bench-ws-5k: ## WebSocket 5k 连接压测（中等规模）
	@echo "$(CYAN)>>> 启动 5k WebSocket 压测$(NC)"
	@echo "配置: 5 Pod × 1000 连接 = 5000 并发"
	@echo "持续: 5 分钟, 爬坡: 1 分钟"
	@echo ""
	@chmod +x $(BENCH_DIR)/scripts/bench-ws.sh
	@$(BENCH_DIR)/scripts/bench-ws.sh 5000 5 1000 5m 1m

bench-ws-10k: ## WebSocket 10k 连接压测（稳定可达）
	@echo "$(CYAN)>>> 启动 10k WebSocket 压测$(NC)"
	@echo "配置: 10 Pod × 1000 连接 = 10000 并发"
	@echo "持续: 5 分钟, 爬坡: 1 分钟"
	@echo ""
	@chmod +x $(BENCH_DIR)/scripts/bench-ws.sh
	@$(BENCH_DIR)/scripts/bench-ws.sh 10000 10 1000 5m 1m

bench-ws-50k: ## WebSocket 50k 连接压测（Docker Desktop 挑战）
	@echo "$(CYAN)>>> 启动 50k WebSocket 压测$(NC)"
	@echo "$(YELLOW)警告: Docker Desktop 单机环境下 50k 连接存在资源瓶颈$(NC)"
	@echo "配置: 20 Pod × 2500 连接 = 50000 并发"
	@echo "持续: 10 分钟, 爬坡: 2 分钟"
	@echo ""
	@echo "推荐先运行 bench-ws-10k 验证基础能力"
	@read -p "是否继续? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo ""
	@chmod +x $(BENCH_DIR)/scripts/bench-ws.sh
	@$(BENCH_DIR)/scripts/bench-ws.sh 50000 20 2500 10m 2m

bench-msg-throughput: ## 消息吞吐量压测（固定连接数，压消息）
	@echo "$(CYAN)>>> 启动消息吞吐量压测$(NC)"
	@echo "配置: 5000 连接, 每连接 10 msg/s"
	@echo "持续: 5 分钟, 爬坡: 1 分钟"
	@echo ""
	@chmod +x $(BENCH_DIR)/scripts/bench-msg.sh
	@$(BENCH_DIR)/scripts/bench-msg.sh 5000 10 5m 1m

bench-collect: ## 收集压测数据（环境信息+指标+日志+HPA）
	@echo "$(CYAN)>>> 收集压测数据...$(NC)"
	@mkdir -p $(RESULTS_DIR)
	@chmod +x $(DEPLOY_DIR)/scripts/collect.sh
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S) && \
		echo "结果目录: $(RESULTS_DIR)/$$TIMESTAMP" && \
		$(DEPLOY_DIR)/scripts/collect.sh $(NAMESPACE) $(RESULTS_DIR)/$$TIMESTAMP && \
		echo "" && \
		echo "$(GREEN)>>> 数据收集完成: $(RESULTS_DIR)/$$TIMESTAMP$(NC)"

bench-stop: ## 停止所有压测
	@echo "$(YELLOW)>>> 停止压测...$(NC)"
	kubectl scale deployment/wsbench -n $(NAMESPACE) --replicas=0 2>/dev/null || true
	@echo "$(GREEN)>>> 压测已停止$(NC)"

bench-local: ## 本地运行压测（直接连接 NodePort，不依赖 K8s wsbench Pod）
	@echo "$(CYAN)>>> 本地运行 wsbench（1000 连接）$(NC)"
	@if [ ! -f "$(BENCH_DIR)/wsbench/wsbench" ]; then \
		echo "$(YELLOW)>>> wsbench 未编译，开始编译...$(NC)"; \
		cd $(BENCH_DIR)/wsbench && go build -o wsbench .; \
	fi
	cd $(BENCH_DIR)/wsbench && ./wsbench \
		--target=ws://localhost:30084/ws \
		--conns=1000 \
		--duration=2m \
		--ramp=30s \
		--mode=connect-only

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

install-metrics-server: ## 安装 metrics-server（适配 Docker Desktop）
	@echo "$(CYAN)>>> 安装 metrics-server...$(NC)"
	@chmod +x $(DEPLOY_DIR)/scripts/install-metrics-server.sh
	@$(DEPLOY_DIR)/scripts/install-metrics-server.sh

verify-metrics: ## 验证 metrics-server 是否工作
	@echo "$(CYAN)>>> 验证 metrics-server 状态...$(NC)"
	@echo ""
	@echo "--- Deployment 状态 ---"
	@kubectl get deployment metrics-server -n kube-system 2>/dev/null || echo "$(RED)metrics-server 未安装$(NC)"
	@echo ""
	@echo "--- API 可用性 ---"
	@if kubectl top nodes &>/dev/null; then \
		echo "$(GREEN)✓ Metrics API 可用$(NC)"; \
		echo ""; \
		kubectl top nodes; \
	else \
		echo "$(RED)✗ Metrics API 不可用$(NC)"; \
		echo ""; \
		echo "排障建议:"; \
		echo "  1. 运行: make install-metrics-server"; \
		echo "  2. 检查日志: kubectl logs -n kube-system -l k8s-app=metrics-server"; \
	fi

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
