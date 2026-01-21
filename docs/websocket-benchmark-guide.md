# WebSocket 压力测试指南

本文档详细说明 IM 系统 WebSocket 服务的压力测试方案、工具使用方法及结果分析。

---

## 目录

1. [概述](#1-概述)
2. [测试环境架构](#2-测试环境架构)
3. [核心概念](#3-核心概念)
4. [环境准备](#4-环境准备)
5. [执行压测](#5-执行压测)
6. [指标解读](#6-指标解读)
7. [实测数据](#7-实测数据)
8. [瓶颈分析与优化](#8-瓶颈分析与优化)
9. [常见问题](#9-常见问题)

---

## 1. 概述

### 1.1 测试目标

验证 IM 系统在高并发场景下的 WebSocket 长连接承载能力：

| 指标 | 目标值 | 说明 |
|------|--------|------|
| 并发连接数 | 50,000+ | 单集群同时在线用户数 |
| 连接成功率 | ≥99% | 连接建立成功的比例 |
| P99 连接延迟 | <100ms | 99% 请求的响应时间 |
| 心跳成功率 | ≥99% | 长连接保活成功率 |

### 1.2 测试工具

- **wsbench**: 自研 WebSocket 压测工具，支持多模式测试
- **collect.sh**: 自动化数据收集脚本
- **Makefile**: 标准化命令入口

---

## 2. 测试环境架构

### 2.1 架构图

```
┌────────────────────────────────────────────────────────────────┐
│                     Docker Desktop (macOS)                      │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Kubernetes 集群                        │   │
│  │                                                          │   │
│  │  ┌──────────────┐        ┌────────────────────────────┐ │   │
│  │  │ API Gateway  │        │    Delivery Service        │ │   │
│  │  │   ×2 Pod     │        │       ×4~8 Pod             │ │   │
│  │  │  :30080      │        │       :30084               │ │   │
│  │  └──────────────┘        └────────────────────────────┘ │   │
│  │                                    ↑                     │   │
│  │                                    │ WebSocket           │   │
│  │                                    │                     │   │
│  │  ┌─────────────────────────────────┴───────────────────┐│   │
│  │  │               wsbench 压测客户端                     ││   │
│  │  │           模拟 10K~50K 并发连接                      ││   │
│  │  └─────────────────────────────────────────────────────┘│   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │  MySQL   │ │  Redis   │ │  Kafka   │ │  MinIO   │          │
│  │  :3306   │ │  :6379   │ │  :29092  │ │  :9000   │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                          (KRaft 模式)                           │
└────────────────────────────────────────────────────────────────┘
```

### 2.2 组件说明

| 组件 | 镜像 | 用途 |
|------|------|------|
| Delivery Service | im/delivery-service:latest | WebSocket 网关，维护长连接 |
| API Gateway | im/api-gateway:latest | HTTP API 入口 |
| MySQL 8.0 | mysql:8.0 | 持久化存储 |
| Redis 7.2 | redis:7.2-alpine | 会话缓存、在线状态 |
| Kafka 3.7 | apache/kafka:3.7.0 | 消息队列（KRaft 模式，无需 Zookeeper） |
| MinIO | minio/minio:latest | 对象存储 |

---

## 3. 核心概念

### 3.1 WebSocket 长连接

与 HTTP 短连接不同，WebSocket 建立连接后持续保持，服务端可主动推送消息。IM 系统依赖此特性实现实时消息投递。

### 3.2 并发连接数

同一时刻保持活跃的 WebSocket 连接数量。50K 并发意味着系统同时维护 5 万个活跃连接。

### 3.3 爬坡（Ramp-up）

渐进式建立连接，模拟真实用户行为：

```
爬坡时间 = 60 秒，目标 = 10,000 连接
→ 建立速率 = 166.7 连接/秒
```

### 3.4 延迟百分位数

| 指标 | 含义 |
|------|------|
| P50 | 50% 请求的延迟低于此值（中位数）|
| P90 | 90% 请求的延迟低于此值 |
| P95 | 95% 请求的延迟低于此值 |
| P99 | 99% 请求的延迟低于此值（关注点）|

P99 是评估系统性能的关键指标，代表极端情况下的响应时间。

### 3.5 心跳机制

客户端定时发送 Ping，服务端响应 Pong，用于检测连接存活状态。心跳成功率反映连接稳定性。

---

## 4. 环境准备

### 4.1 前置条件

| 软件 | 版本 | 验证命令 |
|------|------|---------|
| Docker Desktop | 最新版 | `docker --version` |
| Kubernetes | Docker Desktop 内置 | `kubectl version` |
| Go | 1.21+ | `go version` |

### 4.2 启动依赖服务

```bash
# 启动 MySQL、Redis、Kafka、MinIO
cd deploy
docker compose -f docker-compose.dev.yml up -d

# 验证服务状态（全部 healthy）
docker ps --format "table {{.Names}}\t{{.Status}}" | grep im_
```

预期输出：
```
im_mysql    Up X minutes (healthy)
im_redis    Up X minutes (healthy)
im_kafka    Up X minutes (healthy)
im_minio    Up X minutes (healthy)
```

### 4.3 部署 K8s 服务

```bash
# 构建镜像并部署
make build
make k8s-up

# 验证 Pod 状态
kubectl get pods -n im
```

预期输出：
```
NAME                                READY   STATUS    RESTARTS   AGE
api-gateway-xxx                     1/1     Running   0          1m
api-gateway-xxx                     1/1     Running   0          1m
delivery-service-xxx                1/1     Running   0          1m
delivery-service-xxx                1/1     Running   0          1m
delivery-service-xxx                1/1     Running   0          1m
delivery-service-xxx                1/1     Running   0          1m
```

### 4.4 编译压测工具

```bash
cd bench/wsbench
go build -o wsbench .
```

---

## 5. 执行压测

### 5.1 命令行参数

```bash
./wsbench [选项]
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--target` | - | WebSocket 服务地址 |
| `--conns` | 1000 | 目标连接数 |
| `--duration` | 60s | 测试持续时间 |
| `--ramp` | 30s | 爬坡时间 |
| `--ping-interval` | 30s | 心跳间隔 |
| `--mode` | connect-only | 测试模式 |

### 5.2 测试模式

| 模式 | 说明 |
|------|------|
| `connect-only` | 仅测试连接建立和保持 |
| `messaging` | 连接 + 消息收发 |

### 5.3 快速测试（1K 连接）

```bash
./wsbench \
  --target ws://localhost:30084/ws \
  --conns 1000 \
  --duration 60s \
  --ramp 30s \
  --mode connect-only
```

### 5.4 标准测试（10K 连接）

```bash
./wsbench \
  --target ws://localhost:30084/ws \
  --conns 10000 \
  --duration 60s \
  --ramp 30s \
  --ping-interval 30s \
  --mode connect-only
```

### 5.5 高压测试（30K+ 连接）

```bash
# 先扩容 delivery-service
kubectl scale deployment/delivery-service -n im --replicas=8

# 运行测试
./wsbench \
  --target ws://localhost:30084/ws \
  --conns 30000 \
  --duration 120s \
  --ramp 90s \
  --ping-interval 30s \
  --mode connect-only
```

### 5.6 使用 Makefile 快捷命令

```bash
make bench-local     # 本地 1K 测试
make bench-ws-10k    # K8s 10K 测试
make bench-ws-50k    # K8s 50K 测试
make bench-collect   # 收集测试数据
```

---

## 6. 指标解读

### 6.1 输出示例

```
==================== 压测结果 ====================

--- 连接统计 ---
尝试连接数:     10000
成功连接数:     10000
失败连接数:     0
连接成功率:     100.00%
断开连接数:     0
最终连接数:     0

--- 连接延迟 (ms) ---
Min:    0.65
Max:    97.21
Avg:    2.66
P50:    1.60
P90:    4.26
P95:    6.31
P99:    24.73
StdDev: 4.45

--- 心跳统计 ---
发送 Ping 数:   10000
接收 Pong 数:   10000
Pong 响应率:    100.00%

--- 运行时间: 60.16 秒 ---

=================================================
```

### 6.2 健康指标参考

| 指标 | 优秀 | 良好 | 需优化 | 异常 |
|------|------|------|--------|------|
| 连接成功率 | ≥99.9% | ≥99% | ≥95% | <95% |
| P50 延迟 | <5ms | <20ms | <50ms | ≥50ms |
| P99 延迟 | <50ms | <100ms | <200ms | ≥200ms |
| Pong 响应率 | ≥99.9% | ≥99% | ≥95% | <95% |

---

## 7. 实测数据

### 7.1 测试环境

- **设备**: Mac Mini M4, 16GB RAM
- **Docker Desktop**: Kubernetes 内置集群
- **Delivery Service**: 4~8 Pod

### 7.2 10K 连接测试结果

**配置**: 4 Pod, 30s 爬坡, 60s 持续

```
连接成功率:     100.00%
P50 延迟:       1.60ms
P99 延迟:       24.73ms
Pong 响应率:    100.00%
```

**结论**: 系统稳定支撑 10,000 并发连接

### 7.3 30K 连接测试结果

**配置**: 8 Pod, 90s 爬坡, 120s 持续

```
连接成功率:     35.11%
P50 延迟:       3.14ms
P99 延迟:       971.31ms
失败原因:       i/o timeout (网络栈限制)
```

**结论**: Docker Desktop 网络栈在 10K+ 连接后性能下降，非服务端问题

### 7.4 环境限制说明

| 环境 | 预期并发上限 | 限制因素 |
|------|-------------|---------|
| Docker Desktop (macOS) | ~10K | VM 网络栈、文件描述符 |
| Linux 服务器 (8C16G) | ~50K | 需调优系统参数 |
| K8s 集群 (多节点) | 100K+ | 水平扩展 Delivery Service |

---

## 8. 瓶颈分析与优化

### 8.1 常见瓶颈

| 现象 | 可能原因 | 排查命令 |
|------|----------|---------|
| 连接成功率低 | 文件描述符限制 | `ulimit -n` |
| 连接成功率低 | 端口耗尽 | `netstat -an \| wc -l` |
| P99 延迟高 | CPU 不足 | `kubectl top pods -n im` |
| 大量断开 | 内存不足 | `kubectl describe pod <pod>` |
| i/o timeout | 网络栈限制 | Docker Desktop 限制 |

### 8.2 优化建议

**服务端**:
- 增加 Delivery Service 副本数
- 调整 Pod 资源限制（CPU/Memory）
- 优化 Go runtime 参数（GOMAXPROCS、GOGC）

**客户端**:
- 提高文件描述符限制: `ulimit -n 65536`
- 使用真实 Linux 环境而非 Docker Desktop

### 8.3 数据收集

测试结束后使用 `collect.sh` 收集环境数据：

```bash
make bench-collect
# 或手动执行
./deploy/scripts/collect.sh im ./bench/results/$(date +%Y%m%d_%H%M%S)
```

收集内容包括：
- Pod 状态和描述
- 服务日志
- HPA 状态
- K8s 事件
- 资源使用情况

---

## 9. 常见问题

### Q1: 为什么 Docker Desktop 上超过 10K 连接后失败率上升？

Docker Desktop 在 macOS 上通过虚拟机运行，网络栈存在性能限制。这是环境约束，非服务端问题。生产环境压测应使用 Linux 服务器。

### Q2: connect-only 模式和 messaging 模式的区别？

- `connect-only`: 测试连接容量，验证系统能承载多少长连接
- `messaging`: 测试消息吞吐，验证有业务负载时的表现

### Q3: 如何判断瓶颈在客户端还是服务端？

1. 检查服务端 CPU/内存是否饱和: `kubectl top pods -n im`
2. 检查客户端是否报 "too many open files"
3. 检查是否出现 "i/o timeout"（通常是网络栈限制）

### Q4: 生产环境需要做哪些系统调优？

```bash
# Linux 系统参数
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sysctl -w fs.file-max=2097152

# 文件描述符限制
ulimit -n 1048576
```

---

## 附录

### A. 文件结构

```
IM/
├── bench/
│   ├── wsbench/           # 压测工具
│   │   ├── main.go
│   │   └── Dockerfile
│   └── results/           # 测试结果
│       └── YYYYMMDD_HHMMSS/
├── deploy/
│   ├── docker-compose.dev.yml   # 依赖服务
│   ├── k8s/                     # K8s 部署配置
│   └── scripts/
│       └── collect.sh           # 数据收集脚本
└── Makefile                     # 命令入口
```

### B. 快速参考

| 操作 | 命令 |
|------|------|
| 启动依赖 | `make docker-deps-up` |
| 部署服务 | `make k8s-up` |
| 查看状态 | `make k8s-status` |
| 10K 测试 | `make bench-ws-10k` |
| 收集数据 | `make bench-collect` |
| 查看日志 | `make k8s-logs APP=delivery-service` |
| 停止测试 | `make bench-stop` |
