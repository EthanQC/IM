# 构建与部署工具说明

本文档说明项目中 Makefile 和自动化脚本的使用方法。

---

## Makefile 命令参考

项目使用 Makefile 作为标准构建入口，提供以下命令分类：

### 查看帮助

```bash
make help
```

输出所有可用命令及说明。

---

## 1. 构建命令

| 命令 | 说明 |
|------|------|
| `make build` | 构建所有 Docker 镜像 |
| `make build-gateway` | 构建 API Gateway 镜像 |
| `make build-delivery` | 构建 Delivery Service 镜像 |
| `make build-wsbench` | 构建压测工具镜像 |

### 示例

```bash
# 构建单个服务
make build-delivery

# 重新构建所有镜像
make build
```

---

## 2. Kubernetes 部署命令

| 命令 | 说明 |
|------|------|
| `make k8s-up` | 部署所有服务到 K8s |
| `make k8s-down` | 删除所有 K8s 资源 |
| `make k8s-status` | 查看部署状态 |
| `make k8s-logs APP=<name>` | 查看指定服务日志 |
| `make k8s-restart APP=<name>` | 重启指定服务 |

### 示例

```bash
# 部署
make k8s-up

# 查看 delivery-service 日志
make k8s-logs APP=delivery-service

# 重启 API Gateway
make k8s-restart APP=api-gateway

# 查看所有 Pod 状态
make k8s-status
```

---

## 3. 压测命令

| 命令 | 说明 |
|------|------|
| `make bench-local` | 本地直接运行压测（1K 连接）|
| `make bench-ws-10k` | K8s 集群 10K 连接压测 |
| `make bench-ws-50k` | K8s 集群 50K 连接压测 |
| `make bench-ws-msg` | 消息模式压测 |
| `make bench-collect` | 收集压测数据 |
| `make bench-stop` | 停止压测 |

### 示例

```bash
# 本地快速验证
make bench-local

# 完整 10K 压测
make bench-ws-10k

# 收集测试环境数据
make bench-collect
```

---

## 4. 依赖服务管理

| 命令 | 说明 |
|------|------|
| `make docker-deps-up` | 启动依赖服务（MySQL/Redis/Kafka/MinIO）|
| `make docker-deps-down` | 停止依赖服务 |
| `make docker-deps-status` | 查看依赖服务状态 |

### 示例

```bash
# 启动所有依赖
make docker-deps-up

# 检查状态
make docker-deps-status
```

---

## 5. 完整流程命令

| 命令 | 说明 |
|------|------|
| `make full-deploy` | 完整部署流程：启动依赖 → 构建 → 部署 |
| `make full-bench` | 完整压测流程：运行测试 → 收集数据 |

### 示例

```bash
# 从零开始部署并测试
make full-deploy
make full-bench
```

---

## 6. 其他命令

| 命令 | 说明 |
|------|------|
| `make clean` | 清理构建产物和镜像 |
| `make install-metrics-server` | 安装 metrics-server（用于 kubectl top）|

---

## 脚本说明

### deploy/scripts/collect.sh

数据收集脚本，自动收集 K8s 环境信息用于问题排查。

**用法**:
```bash
./deploy/scripts/collect.sh [namespace] [output_dir]

# 示例
./deploy/scripts/collect.sh im ./bench/results/$(date +%Y%m%d_%H%M%S)
```

**收集内容**:

| 文件 | 内容 |
|------|------|
| `cluster-info.txt` | K8s 集群信息 |
| `nodes.txt` | 节点列表 |
| `pods.txt` / `pods.yaml` | Pod 状态 |
| `services.txt` | Service 列表 |
| `hpa.txt` | HPA 状态 |
| `top-pods.txt` | 资源使用情况 |
| `describe-*.txt` | Pod 详细描述 |
| `logs/` | 各服务日志 |
| `metrics/` | 应用指标 |
| `events.txt` | K8s 事件 |
| `summary.txt` | 汇总信息 |

### deploy/scripts/deploy.sh

生产环境部署脚本（待实现）。

### deploy/scripts/server-init.sh

服务器初始化脚本，用于新服务器环境配置。

---

## Docker Compose 配置

### deploy/docker-compose.dev.yml

本地开发环境依赖服务配置：

| 服务 | 端口 | 说明 |
|------|------|------|
| MySQL | 3306 | 数据库 |
| Redis | 6379 | 缓存 |
| Kafka | 29092 | 消息队列（KRaft 模式）|
| MinIO | 9000/9001 | 对象存储 |

**启动**:
```bash
cd deploy
docker compose -f docker-compose.dev.yml up -d
```

**停止**:
```bash
docker compose -f docker-compose.dev.yml down
```

**完全清理（含数据卷）**:
```bash
docker compose -f docker-compose.dev.yml down -v
```

---

## Kustomize 配置

K8s 部署使用 Kustomize 管理配置：

```
deploy/k8s/
├── base/                    # 基础配置
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── api-gateway/
│   └── delivery-service/
└── overlays/
    └── docker-desktop/      # Docker Desktop 专用配置
        ├── kustomization.yaml
        └── patches/
```

**手动部署**:
```bash
kubectl apply -k deploy/k8s/overlays/docker-desktop
```

**手动删除**:
```bash
kubectl delete -k deploy/k8s/overlays/docker-desktop
```

---

## 常用变量

Makefile 中定义的变量可在命令行覆盖：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NAMESPACE` | im | K8s 命名空间 |
| `APP` | - | 指定服务名称 |

**示例**:
```bash
# 查看自定义命名空间的日志
make k8s-logs APP=delivery-service NAMESPACE=production
```
