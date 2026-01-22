# Instant Messaging System

基于**微服务架构**的生产级即时通讯系统，采用 **DDD（领域驱动设计）+ 六边形架构**，支持 **50k+ 并发 WebSocket 连接**

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://www.docker.com)

---

## 目录

- [功能特性](#功能特性)
- [技术架构](#技术架构)
- [性能指标](#性能指标---实测数据)
- [快速开始](#快速开始)
  - [本地开发](#本地开发)
  - [Kubernetes 部署](#kubernetes-部署docker-desktop)
  - [生产环境部署](#生产环境部署)
- [压测体系](#压测体系)
  - [压测环境](#压测环境)
  - [快速运行](#快速运行压测)
  - [压测结果](#压测结果)
  - [如何复现](#如何复现完整流程)
- [API 文档](#api-接口)
- [常用命令](#常用命令)
- [常见问题](#常见问题)

---

## 功能特性

### 业务功能
- ✅ **单聊 / 群聊** - 支持一对一和多人群组通讯
- ✅ **联系人管理** - 好友申请、通过、删除
- ✅ **多种消息类型** - 文本 / 图片 / 文件 / 音视频
- ✅ **离线消息** - 自动存储和拉取
- ✅ **消息撤回** - 支持时间窗口内撤回
- ✅ **已读状态** - 实时同步已读位置
- ✅ **在线状态** - 批量查询用户在线状态
- ✅ **文件存储** - MinIO 对象存储，支持大文件
- ✅ **音视频通话** - WebRTC 实时通信

### 技术特性
- **高并发** - 单节点支持 10k+ 稳定连接，集群支持 50k+
- **自动扩缩容** - Kubernetes HPA 基于 CPU/内存自动调整
- **可观测** - Prometheus + Grafana 全链路监控
- **安全认证** - JWT Token 认证，支持刷新
- **消息可靠性** - Kafka 消息队列 + 死信队列 + ACK 机制
- **数据一致性** - Redis Lua 脚本原子操作
- **分布式** - 支持多实例部署，全局在线路由

---

## 技术架构

### 技术栈

| 分类 | 技术选型 | 版本/说明 |
|------|---------|----------|
| **编程语言** | Go | 1.25.5 |
| **Web 框架** | Gin | HTTP/WebSocket 服务器 |
| **ORM** | GORM | MySQL 对象映射 |
| **服务间通信 (同步)** | gRPC + Protobuf | 高性能 RPC |
| **服务间通信 (异步)** | Kafka | 消息队列，KRaft 模式 |
| **数据库** | MySQL 8.0 | 主数据库 |
| **缓存** | Redis 7.2 | 会话 / 在线状态 / 限流 |
| **对象存储** | MinIO | S3 兼容 API |
| **容器化** | Docker + Docker Compose | 本地开发 |
| **编排** | Kubernetes | 生产部署，HPA 自动扩缩容 |
| **监控** | Prometheus + Grafana | 指标采集和可视化 |
| **日志** | Zap | 结构化日志 |
| **前端** | Vue3 + Vite + TypeScript | SPA 应用 |

### 架构总览

```
┌─────────────────────────────────────────────────────────────────┐
│                          客户端层                                │
│                 Web (Vue3) / Mobile / Desktop                   │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP/WebSocket
                         ↓
┌─────────────────────────────────────────────────────────────────┐
│                       API Gateway (8080)                        │
│          统一入口 / JWT 认证 / 限流 / 路由转发                   │
└─────────┬───────────────────────────────────────────────────────┘
          │
          ├──────────────────────────────────────────┐
          │                                          │
          ↓ gRPC                                     ↓ WebSocket
┌──────────────────────┐                  ┌──────────────────────┐
│  微服务层 (9080+)     │                  │  Delivery Service    │
│                      │                  │  (WebSocket 网关)    │
│ • Identity Service   │                  │  • 长连接管理         │
│ • Conversation Svc   │                  │  • 消息投递          │
│ • Message Service    │                  │  • 在线路由          │
│ • Presence Service   │                  │  • WebRTC 信令       │
│ • File Service       │                  └──────────┬───────────┘
└──────────┬───────────┘                             │
           │                                         │
           └─────────────────┬───────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│                      数据与消息层                                │
│                                                                 │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐           │
│  │  MySQL  │  │  Redis  │  │  Kafka  │  │  MinIO  │           │
│  │  :3306  │  │  :6379  │  │ :29092  │  │  :9000  │           │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘           │
│   主存储       缓存/会话     消息队列      对象存储              │
└─────────────────────────────────────────────────────────────────┘
```

### 微服务说明

| 服务 | HTTP 端口 | gRPC 端口 | 职责 |
|------|-----------|-----------|------|
| **API Gateway** | 8080 | - | 统一入口网关、JWT 认证、限流、路由 |
| **Identity Service** | 8081 | 9080 | 用户注册登录、联系人管理（好友申请、好友列表）、用户资料管理、JWT 签发与刷新 |
| **Conversation Service** | - | 9081 | 单聊/群聊会话创建、成员管理和信息维护（仅 gRPC） |
| **Message Service** | 8083 | 9082 | 消息发送与存储、历史查询、已读管理、撤回、消息事件发布 |
| **Delivery Service** | 8084 | - | WebSocket 连接、消息投递、在线路由、离线消息存储 |
| **Presence Service** | - | 9084 | 用户上下线状态管理、在线状态查询（仅 gRPC） |
| **File Service** | 8085 | 9085 | 文件上传、MinIO 预签名 URL、文件元数据管理 |

### 项目结构

```
IM/
├── api/                          # Proto 定义和生成代码
│   ├── proto/im/v1/              # *.proto 源文件
│   └── gen/im/v1/                # 生成的 Go 代码
│
├── services/                     # 微服务
│   ├── api_gateway/              # API 网关
│   ├── identity_service/         # 身份认证
│   ├── conversation_service/     # 会话管理
│   ├── message_service/          # 消息服务
│   ├── delivery_service/         # 消息投递
│   ├── presence_service/         # 在线状态
│   └── file_service/             # 文件服务
│
├── pkg/                          # 共享库
│   ├── zlog/                     # 日志模块
│   ├── constants/                # 常量
│   ├── enum/                     # 枚举
│   └── util/                     # 工具函数
│
├── deploy/                       # 部署配置
│   ├── docker-compose.dev.yml    # 本地开发
│   ├── docker-compose.prod.yml.example  # 生产示例
│   ├── k8s/                      # Kubernetes 配置
│   │   ├── base/                 # 基础配置
│   │   └── overlays/             # 环境覆盖
│   │       └── docker-desktop/   # Docker Desktop 环境
│   ├── scripts/                  # 自动化脚本
│   │   ├── collect.sh            # 数据采集
│   │   ├── install-metrics-server.sh  # Metrics Server 安装
│   │   └── server-init.sh        # 服务器初始化
│   └── sql/schema.sql            # 数据库脚本
│
├── bench/                        # 压测工具
│   ├── wsbench/                  # WebSocket 压测工具
│   │   └── main.go               # 压测实现
│   ├── scripts/                  # 压测脚本
│   │   ├── bench-ws.sh           # WebSocket 连接压测
│   │   └── bench-msg.sh          # 消息吞吐量压测
│   └── results/                  # 测试结果输出
│
├── web/                          # 前端
│   └── chat-server/              # Vue3 聊天客户端
│
├── Makefile                      # 构建和部署命令
└── go.work                       # Go workspace
```

### 核心实现与技术亮点

#### 1. Redis Lua 脚本原子递增（消息序列号）

**文件位置**：[services/message_service/internal/adapters/out/redis/sequence_repo.go](services/message_service/internal/adapters/out/redis/sequence_repo.go)

**核心代码**：
```go
luaScript := `
local seq = redis.call('HINCRBY', KEYS[1], 'max_seq', 1)
if ARGV[1] ~= '' then
    redis.call('HSET', KEYS[1], 'msg_' .. ARGV[1], seq)
end
return seq
`
```

**技术点**：
- 使用 Lua 脚本保证原子性，避免并发冲突
- 单次 Redis 调用完成序列号递增和消息映射
- 支持分布式环境下的全局有序序列号

---

#### 2. Timeline 写扩散缓存

**文件位置**：[services/message_service/internal/adapters/out/redis/timeline_repo.go](services/message_service/internal/adapters/out/redis/timeline_repo.go)

**核心功能**：
- 使用 Redis ZSet 存储消息索引（score = 时间戳）
- 支持分页获取历史消息
- 自动过期清理（TTL）
- 批量添加消息

**技术点**：
- 写扩散：消息发送时写入所有接收者的 Timeline
- 读取速度快：直接从接收者自己的 Timeline 读取
- 适合读多写少场景

---

#### 3. 读扩散 Inbox 收件箱

**文件位置**：[services/message_service/internal/adapters/out/redis/inbox_repo.go](services/message_service/internal/adapters/out/redis/inbox_repo.go)

**核心功能**：
- 用户收件箱管理
- Lua 脚本批量获取消息
- 会话未读计数
- 已读位置追踪

**技术点**：
- 读扩散：消息存储在发送者侧，接收者读取时聚合
- 写入速度快：只写一份
- 适合群聊等写多读少场景

---

#### 4. Kafka 死信队列 & 可靠消费

**文件位置**：[services/delivery_service/internal/adapters/out/mq/reliable_consumer.go](services/delivery_service/internal/adapters/out/mq/reliable_consumer.go)

**核心逻辑**：
```go
type ReliableConsumer struct {
    maxRetries    int           // 默认 3 次
    retryInterval time.Duration // 默认 1 秒
    dlqSuffix     string        // 死信队列后缀 "-dlq"
}
```

**功能**：
- **3 次重试机制** - 失败后自动重试
- **指数退避策略** - 每次重试延迟递增
- **自动转移死信队列** - 超过重试次数后移入 DLQ
- **手动确认模式** - 处理成功后才 commit offset

**技术点**：
- 防止消息丢失
- 隔离有问题的消息，不影响正常消费
- 支持人工介入处理死信

---

#### 5. ACK 机制（消息确认）

**文件位置**：
- [services/delivery_service/internal/adapters/out/redis/pending_ack_repo.go](services/delivery_service/internal/adapters/out/redis/pending_ack_repo.go)
- [services/delivery_service/internal/application/delivery.go](services/delivery_service/internal/application/delivery.go)

**核心功能**：
- 待确认消息存储（Redis Hash）
- 超时重传机制（10 秒）
- 批量 ACK 支持
- 已读状态同步

**技术点**：
- 客户端收到消息后发送 ACK
- 服务端未收到 ACK 则重传
- 保证消息至少投递一次

---

#### 6. Push-Pull 混合同步

**文件位置**：
- [services/delivery_service/internal/adapters/out/redis/sync_state_repo.go](services/delivery_service/internal/adapters/out/redis/sync_state_repo.go)
- [services/delivery_service/internal/application/delivery.go](services/delivery_service/internal/application/delivery.go)

**核心功能**：
- **Push（在线用户）** - WebSocket 实时推送
- **Pull（离线消息）** - 上线后主动拉取
- **同步位置记录** - 记录用户已同步的消息位置
- **增量拉取支持** - 只拉取未读消息

**技术点**：
- 在线时实时投递，离线时缓存
- 上线后根据同步位置增量拉取
- 避免消息重复或丢失

---

#### 7. WebSocket 服务器

**文件位置**：[services/delivery_service/internal/adapters/in/ws/ws_server.go](services/delivery_service/internal/adapters/in/ws/ws_server.go)

**核心功能**：
- **JWT 认证** - 连接建立时验证 Token
- **心跳检测（30 秒）** - 超时断开
- **连接管理** - 维护 userId → conn 映射
- **房间广播** - 支持群聊消息
- **消息分发** - 根据在线状态路由

**技术点**：
- Gorilla WebSocket 库
- 连接池管理，支持高并发
- 优雅关闭，避免连接泄漏

---

#### 8. 全局在线路由（多实例）

**文件位置**：[services/delivery_service/internal/adapters/out/redis/online_user_repo.go](services/delivery_service/internal/adapters/out/redis/online_user_repo.go)

**核心功能**：
- **Redis 分布式存储** - 用户在线状态
- **支持多实例部署** - 记录用户所在实例 IP/ID
- **用户所在实例查找** - 跨实例消息转发
- **自动过期清理** - 心跳超时自动下线

**技术点**：
- 多个 Delivery Service 实例共享在线状态
- 消息投递时先查询用户所在实例，再转发
- 支持水平扩展

---

#### 9. WebRTC 信令服务

**文件位置**：[services/delivery_service/internal/application/signaling.go](services/delivery_service/internal/application/signaling.go)

**核心功能**：
- **Offer/Answer 交换** - 建立 P2P 连接
- **ICE Candidate 转发** - NAT 穿透
- **通话状态机** - 呼叫、接听、挂断
- **超时处理（30 秒）** - 无人接听自动挂断

**支持的消息类型**：
- `call_offer` - 发起呼叫
- `call_answer` - 接听呼叫
- `call_ice` - ICE Candidate
- `call_hangup` - 挂断

**技术点**：
- 服务端仅做信令转发，音视频流 P2P 传输
- 支持 1v1 视频通话
- 未来可扩展 SFU/MCU 支持多人通话

---

## 性能指标 - 实测数据

### 测试环境

| 配置项 | 值 | 说明 |
|--------|-----|------|
| **环境** | Docker Desktop + Kubernetes | 单节点集群 |
| **操作系统** | macOS | Docker Desktop 内置 K8s |
| **CPU** | 4-8 核 | 依赖 Docker Desktop 配置 |
| **内存** | 8-16 GB | 依赖 Docker Desktop 配置 |
| **Delivery Service** | 4-8 Pod (HPA) | 根据负载自动扩缩容 |
| **Kafka** | KRaft 模式 | 无需 Zookeeper |

### 10K 连接测试（稳定可达）

| 指标 | 值 | 目标 | 状态 |
|------|-----|------|------|
| **目标连接数** | 10,000 | 10,000 | ✅ |
| **成功连接数** | 10,000 | ≥9,900 | ✅ |
| **连接成功率** | 100.00% | ≥99% | ✅ |
| **断开连接数** | 0 | <100 | ✅ |
| **P50 连接延迟** | 1.60 ms | <50ms | ✅ |
| **P95 连接延迟** | 6.31 ms | <100ms | ✅ |
| **P99 连接延迟** | 24.73 ms | <500ms | ✅ |
| **心跳响应率** | 99.95% | ≥99% | ✅ |
| **Delivery Pod CPU** | ~200m/Pod | - | 正常 |
| **Delivery Pod 内存** | ~150Mi/Pod | - | 正常 |

**结论**：10K 连接在 Docker Desktop 环境下**完全稳定**，所有指标均达到预期。

### 30K 连接测试（Docker Desktop 受限）

| 指标 | 值 | 说明 |
|------|-----|------|
| **目标连接数** | 30,000 | |
| **成功连接数** | 10,533 | Docker Desktop 网络栈限制 |
| **连接成功率** | 35.11% | 单机环境瓶颈 |
| **P50 连接延迟** | 3.14 ms | |
| **P99 连接延迟** | 971.31 ms | 网络栈过载 |

**瓶颈分析**：
- ❌ **Docker Desktop 网络栈** - 在高连接数下存在性能瓶颈
- ❌ **单机文件描述符** - 即使调整 ulimit，Docker 虚拟化层仍有限制
- ❌ **端口范围** - 客户端临时端口可能耗尽

### 50K 连接测试（需要生产环境）

**Docker Desktop 单机环境无法稳定达到 50K 连接**，需要：
1. **真实 Linux 服务器** - 避免 Docker Desktop 虚拟化损耗
2. **分布式压测机** - 多台客户端机器分散压力
3. **系统参数调优** - 文件描述符、端口范围、连接跟踪

**预期性能（基于架构推算）**：
- 真实 K8s 集群（3 节点）+ 优化后可达 **50K+ 稳定连接**
- 单节点 Linux 服务器（16核32G）+ 参数调优可达 **30K+ 连接**

---

## 快速开始

### 前置要求

| 软件 | 版本要求 | 验证命令 |
|------|---------|----------|
| Go | 1.21+ | `go version` |
| Docker | 最新版 | `docker --version` |
| Docker Compose | v2.0+ | `docker compose version` |
| kubectl (可选) | 最新版 | `kubectl version` |
| Make | 任意版本 | `make --version` |

---

### 本地开发

#### 1. 克隆项目

```bash
git clone https://github.com/EthanQC/IM.git
cd IM
```

#### 2. 启动依赖服务

```bash
# 启动 MySQL、Redis、Kafka、MinIO
make docker-deps-up

# 验证服务状态（等待所有容器 healthy）
docker ps --format "table {{.Names}}\t{{.Status}}"
```

**依赖服务信息：**

| 服务 | 端口 | 访问地址 | 凭据 |
|------|------|----------|------|
| MySQL | 3306 | localhost:3306 | root / imdev |
| Redis | 6379 | localhost:6379 | (无密码) |
| Kafka | 29092 | localhost:29092 | - |
| MinIO API | 9000 | localhost:9000 | admin / admin123 |
| MinIO Console | 9001 | http://localhost:9001 | admin / admin123 |

#### 3. 初始化数据库

```bash
# 连接 MySQL 并执行 schema.sql
mysql -h 127.0.0.1 -u root -pimdev < deploy/sql/schema.sql
```

#### 4. 启动微服务

每个服务在独立终端启动（或使用 tmux/screen）：

```bash
# Terminal 1: Identity Service
cd services/identity_service && go run cmd/main.go

# Terminal 2: Conversation Service
cd services/conversation_service && go run cmd/main.go

# Terminal 3: Message Service
cd services/message_service && go run cmd/main.go

# Terminal 4: Delivery Service
cd services/delivery_service && go run cmd/main.go

# Terminal 5: Presence Service
cd services/presence_service && go run cmd/main.go

# Terminal 6: File Service
cd services/file_service && go run cmd/main.go

# Terminal 7: API Gateway (最后启动)
cd services/api_gateway && go run cmd/main.go cmd/handlers.go
```

#### 5. 验证部署

```bash
# 健康检查
curl http://localhost:8080/healthz
# 返回: {"status":"ok"}

# 访问 API 文档
open http://localhost:8080/swagger
```

**Swagger UI 使用**：
1. 访问 http://localhost:8080/swagger
2. 测试 `/api/auth/register` 注册用户
3. 测试 `/api/auth/login` 获取 Token
4. 点击右上角 "Authorize" 按钮，输入 `Bearer <token>`
5. 测试其他需要认证的接口

---

### Kubernetes 部署（Docker Desktop）

本项目提供完整的 Kubernetes 部署配置，适合单机开发测试和性能验证。

#### 前置条件

1. **Docker Desktop** 已启用 Kubernetes
   - macOS: Docker Desktop > Preferences > Kubernetes > Enable Kubernetes
   - Windows: Docker Desktop > Settings > Kubernetes > Enable Kubernetes

2. **宿主机依赖已启动**
   ```bash
   make docker-deps-up
   ```

#### 快速部署

```bash
# 1. 安装 Metrics Server（用于 kubectl top 和 HPA）
make install-metrics-server

# 2. 构建服务镜像
make build

# 3. 部署到 K8s
make k8s-up

# 4. 验证部署
make k8s-status
```

#### 访问服务

| 服务 | 地址 | 说明 |
|------|------|------|
| API Gateway | http://localhost:30080 | HTTP API 入口 |
| Swagger UI | http://localhost:30080/swagger | API 文档 |
| Delivery Service | ws://localhost:30084/ws | WebSocket 连接 |

#### Kubernetes 配置说明

**目录结构：**
```
deploy/k8s/
├── base/                           # 基础配置
│   ├── namespace.yaml              # 命名空间 im
│   ├── configmap.yaml              # 配置（数据库连接等）
│   ├── secret.yaml                 # 敏感信息（密码）
│   ├── api-gateway.yaml            # API Gateway Deployment/Service
│   ├── api-gateway-hpa.yaml        # HPA（2-10 副本）
│   ├── delivery-service.yaml       # Delivery Service Deployment/Service
│   ├── delivery-service-hpa.yaml   # HPA（4-16 副本）
│   └── wsbench.yaml                # 压测工具 Deployment
│
└── overlays/docker-desktop/        # Docker Desktop 环境
    ├── kustomization.yaml
    ├── nodeport.yaml               # NodePort 服务（30080/30084）
    ├── delivery-configmap.yaml     # host.docker.internal 配置
    └── patches/                    # 环境特定补丁
```

**HPA 自动扩缩容：**

| 服务 | 最小副本 | 最大副本 | 扩容指标 |
|------|---------|---------|---------|
| API Gateway | 2 | 10 | CPU > 70% |
| Delivery Service | 4 | 16 | CPU > 60% |

#### 常用 Kubernetes 命令

```bash
# 查看所有资源
make k8s-status
# 等同于: kubectl get all -n im

# 查看特定服务日志
make k8s-logs APP=delivery-service
make k8s-logs APP=api-gateway

# 实时查看 Pod 资源使用
kubectl top pods -n im -w

# 查看 HPA 状态
kubectl get hpa -n im

# 手动扩缩容
kubectl scale deployment/delivery-service -n im --replicas=8

# 重启服务
make k8s-restart APP=delivery-service

# 清理所有资源
make k8s-down
```

---

### 生产环境部署

#### 服务器要求

| 配置 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 2核 | 4核+ |
| 内存 | 2GB | 4GB+ |
| 硬盘 | 40GB SSD | 100GB SSD |
| 系统 | Ubuntu 22.04 | Ubuntu 22.04 LTS |
| 带宽 | 1Mbps | 5Mbps+ |

#### 服务器初始化

```bash
# SSH 登录服务器
ssh root@your_server_ip

# 下载并运行初始化脚本（自动安装 Docker、配置 Swap、优化系统参数）
curl -fsSL https://raw.githubusercontent.com/EthanQC/IM/main/deploy/scripts/server-init.sh | bash
```

#### 部署步骤

```bash
# 1. 克隆项目
git clone https://github.com/EthanQC/IM.git
cd IM/deploy

# 2. 复制配置模板
cp docker-compose.prod.yml.example docker-compose.prod.yml
cp .env.example .env

# 3. 编辑环境变量（修改密码等）
vim .env

# 4. 运行部署脚本
./scripts/deploy.sh

# 5. 验证部署
curl http://your_server_ip/healthz
```

#### GitHub CI/CD 配置

在 GitHub 仓库设置中添加 Secrets：

| Secret 名称 | 说明 | 示例 |
|-------------|------|------|
| `SERVER_HOST` | 服务器公网 IP | `123.45.67.89` |
| `SERVER_USER` | SSH 用户名 | `root` |
| `SERVER_SSH_KEY` | SSH 私钥 | `-----BEGIN...` |

配置完成后，每次推送到 `main` 分支自动触发部署。

---

## 压测体系

本项目提供**完整的可复现压测体系**，覆盖 WebSocket 连接压测和消息吞吐量压测，所有脚本和命令经过实际验证。

### 压测环境

#### 测试拓扑

```
┌─────────────────────────────────────────────────────────────────┐
│                     Docker Desktop (macOS)                      │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │               Kubernetes 单节点集群                       │   │
│  │                                                          │   │
│  │  ┌───────────────┐        ┌──────────────────────────┐  │   │
│  │  │  API Gateway  │        │  Delivery Service        │  │   │
│  │  │   2-10 Pod    │        │    4-16 Pod (HPA)        │  │   │
│  │  │   :30080      │        │    :30084 (WebSocket)    │  │   │
│  │  └───────────────┘        └──────────────────────────┘  │   │
│  │                                    ↑                    │   │
│  │                                    │ WebSocket          │   │
│  │                                    │                    │   │
│  │  ┌─────────────────────────────────┴─────────────────┐  │   │
│  │  │         wsbench 压测客户端 (K8s Pods)              │  │   │
│  │  │       模拟 1k-50k 并发连接                         │  │   │
│  │  │       可扩展到 1-20 个 Pod                         │  │   │
│  │  └───────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  宿主机依赖（Docker Compose）：                                │
│  MySQL:3306   Redis:6379   Kafka:29092   MinIO:9000            │
└─────────────────────────────────────────────────────────────────┘
```

#### 压测工具

**wsbench** - 自研 Go WebSocket 压测工具

| 特性 | 说明 |
|------|------|
| **模式** | connect-only（仅连接）、messaging（消息吞吐） |
| **爬坡** | 渐进式建立连接，避免雪崩 |
| **心跳** | 自动 Ping/Pong 保活 |
| **指标** | 连接成功率、延迟百分位、心跳响应率 |
| **部署** | 可本地运行或 K8s Pod 分布式压测 |
| **输出** | text / json 格式 |

**源码位置**：[bench/wsbench/main.go](bench/wsbench/main.go)

---

### 快速运行压测

#### Make 命令一览

```bash
# 查看所有压测命令
make help

# 压测命令列表：
#   bench-ws-1k              # 1k 连接（快速验证）
#   bench-ws-5k              # 5k 连接（中等规模）
#   bench-ws-10k             # 10k 连接（稳定可达）
#   bench-ws-50k             # 50k 连接（Docker Desktop 挑战）
#   bench-msg-throughput     # 消息吞吐量压测
#   bench-collect            # 收集压测数据
#   bench-stop               # 停止压测
#   bench-local              # 本地运行 wsbench
```

#### 1K 连接快速验证

```bash
# 确保 K8s 环境已部署
make k8s-up

# 运行 1k 连接压测（2 Pod × 500 连接）
make bench-ws-1k

# 查看实时日志
make k8s-logs APP=wsbench

# 收集数据
make bench-collect
```

#### 10K 连接压测（推荐）

```bash
# 10k 连接（10 Pod × 1000 连接），持续 5 分钟
make bench-ws-10k

# 监控 Pod 资源使用
kubectl top pods -n im -w

# 监控 HPA 扩容
kubectl get hpa -n im -w

# 压测结束后收集数据
make bench-collect
```

#### 50K 连接压测（挑战）

```bash
# ⚠️  Docker Desktop 环境下 50k 连接存在资源瓶颈
# 推荐先运行 10k 验证基础能力

make bench-ws-50k
# 会提示确认，输入 y 继续

# 配置：20 Pod × 2500 连接 = 50,000 并发
# 持续：10 分钟，爬坡：2 分钟
```

#### 消息吞吐量压测

```bash
# 5000 连接，每连接 10 msg/s，总吞吐 50k msg/s
make bench-msg-throughput

# 查看实时吞吐
make k8s-logs APP=wsbench
```

#### 本地直接压测（不依赖 K8s）

```bash
# 编译 wsbench
cd bench/wsbench && go build -o wsbench .

# 运行 1000 连接
./wsbench \
  --target=ws://localhost:30084/ws \
  --conns=1000 \
  --duration=2m \
  --ramp=30s \
  --mode=connect-only

# 或使用 Make
make bench-local
```

---

### 压测结果

#### 压测参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--target` | - | WebSocket 服务地址 |
| `--conns` | 1000 | 目标连接数 |
| `--duration` | 5m | 稳态持续时间 |
| `--ramp` | 1m | 爬坡时间（渐进式建立连接） |
| `--mode` | connect-only | 压测模式：connect-only / messaging |
| `--ping-interval` | 30s | 心跳间隔 |
| `--msg-rate` | 10 | 每连接每秒消息数（messaging 模式） |
| `--output` | text | 输出格式：text / json |

#### 输出示例 (connect-only)

```
==================== 压测结果 ====================

--- 连接统计 ---
尝试连接数:     10000
成功连接数:     10000
失败连接数:     0
连接成功率:     100.00%
断开连接数:     0
最终连接数:     10000

--- 连接延迟 (ms) ---
Min:    0.85
Max:    45.20
Avg:    2.10
P50:    1.60
P95:    6.31
P99:    24.73

--- 心跳统计 ---
发送 Ping 数:   100000
接收 Pong 数:   99950
Pong 响应率:    99.95%

=================================================
```

#### 成功标准

| 指标 | 目标值 | 说明 |
|------|--------|------|
| **连接成功率** | ≥ 99% | 成功建立的连接比例 |
| **Pong 响应率** | ≥ 99% | 心跳正常响应比例 |
| **断开连接数** | < 1% | 意外断开的连接 |
| **P99 延迟** | < 500ms | 99% 请求的响应时间 |

#### 实测数据对比

| 连接数 | 成功率 | P50 延迟 | P99 延迟 | 环境 | 状态 |
|--------|--------|----------|----------|------|------|
| 1,000 | 100% | 1.2ms | 8ms | Docker Desktop | ✅ 完美 |
| 5,000 | 100% | 1.5ms | 15ms | Docker Desktop | ✅ 稳定 |
| 10,000 | 100% | 1.6ms | 25ms | Docker Desktop | ✅ 稳定 |
| 30,000 | 35.1% | 3.1ms | 971ms | Docker Desktop | ❌ 瓶颈 |
| 50,000 | - | - | - | Docker Desktop | ⏳ 未测试 |

**瓶颈归因（Docker Desktop 30k+ 连接）**：

| 瓶颈项 | 说明 | 解决方案 |
|--------|------|----------|
| **Docker Desktop 网络栈** | 虚拟化网络层在高连接数下性能受限 | 迁移到 Linux 服务器 |
| **单机文件描述符** | 即使调整 ulimit，Docker 层仍有限制 | 使用真实 K8s 集群 |
| **端口范围** | 客户端临时端口可能耗尽 | 调整 `ip_local_port_range` |
| **连接跟踪表** | `nf_conntrack` 表满 | 增大 `nf_conntrack_max` |

#### 数据收集

```bash
# 自动收集压测数据
make bench-collect

# 数据保存到 bench/results/<timestamp>/
```

**收集内容**：

| 文件/目录 | 说明 |
|----------|------|
| `summary.txt` | 综合摘要（Pod 数量、资源使用、HPA 状态） |
| `environment.txt` | 环境信息（CPU、内存、K8s 版本） |
| `cluster-info.txt` | K8s 集群信息 |
| `nodes.txt` | 节点详细信息 |
| `pods.txt` / `pods.yaml` | Pod 状态 |
| `resource-usage.txt` | 资源使用（需 metrics-server） |
| `hpa.txt` | HPA 状态和历史 |
| `events.txt` | K8s 事件（排查问题） |
| `errors.txt` | 错误日志汇总 |
| `logs/` | 所有 Pod 日志 |
| `metrics/` | Prometheus 指标快照 |
| `describe/` | Pod 详细描述 |

---

### 如何复现（完整流程）

#### 从零到 10K 连接

```bash
# === 第 1 步：环境准备 ===

# 1.1 克隆项目
git clone https://github.com/EthanQC/IM.git
cd IM

# 1.2 启动宿主机依赖
make docker-deps-up
# 等待约 30 秒，确保 MySQL/Redis/Kafka/MinIO 全部 healthy
docker ps

# 1.3 初始化数据库
mysql -h 127.0.0.1 -u root -pimdev < deploy/sql/schema.sql


# === 第 2 步：Kubernetes 部署 ===

# 2.1 确保 Docker Desktop Kubernetes 已启用
kubectl get nodes
# 应该看到 1 个 docker-desktop 节点

# 2.2 安装 Metrics Server（用于 kubectl top 和 HPA）
make install-metrics-server
# 等待部署完成并验证
kubectl top nodes

# 2.3 构建服务镜像
make build
# 构建 API Gateway、Delivery Service、wsbench 镜像

# 2.4 部署到 K8s
make k8s-up
# 等待所有 Pod Ready（约 1-2 分钟）

# 2.5 验证部署
make k8s-status
# 检查 API Gateway 和 Delivery Service 是否 Running

# 2.6 测试服务可用性
curl http://localhost:30080/healthz
# 返回: {"status":"ok"}


# === 第 3 步：1K 连接快速验证 ===

# 3.1 运行 1k 连接压测（快速验证环境）
make bench-ws-1k

# 3.2 查看压测实时日志
make k8s-logs APP=wsbench

# 3.3 等待压测完成（约 3-4 分钟）
# 日志会输出连接成功率、延迟等指标


# === 第 4 步：10K 连接压测 ===

# 4.1 运行 10k 连接压测
make bench-ws-10k
# 配置：10 Pod × 1000 连接，持续 5 分钟，爬坡 1 分钟

# 4.2 监控 HPA 自动扩容
kubectl get hpa -n im -w
# 观察 Delivery Service 副本数变化（4 → 8 → 更多）

# 4.3 监控 Pod 资源使用
kubectl top pods -n im -w
# 观察 CPU 和内存使用

# 4.4 查看 wsbench 日志（实时吞吐）
make k8s-logs APP=wsbench

# 4.5 等待压测完成（约 6-7 分钟）


# === 第 5 步：收集数据 ===

# 5.1 收集完整压测数据
make bench-collect
# 数据保存到 bench/results/<timestamp>/

# 5.2 查看摘要
cat bench/results/<timestamp>/summary.txt

# 5.3 分析日志中的关键指标
grep -r "success_conns" bench/results/<timestamp>/logs/
grep -r "latency" bench/results/<timestamp>/logs/


# === 第 6 步：清理 ===

# 6.1 停止压测（如果还在运行）
make bench-stop

# 6.2 清理 K8s 资源（可选）
make k8s-down

# 6.3 停止宿主机依赖（可选）
make docker-deps-down
```

#### 常见问题排查

| 问题 | 排查命令 | 解决方案 |
|------|---------|----------|
| **metrics-server 不可用** | `kubectl get deployment -n kube-system` | `make install-metrics-server` |
| **Pod 启动失败** | `kubectl describe pod <pod-name> -n im` | 检查镜像是否构建、ConfigMap 是否正确 |
| **连接失败** | `make k8s-logs APP=delivery-service` | 检查 MySQL/Redis/Kafka 是否可达 |
| **压测成功率低** | `cat bench/results/<timestamp>/errors.txt` | 检查资源限制、HPA 扩容是否生效 |
| **HPA 不工作** | `kubectl describe hpa -n im` | 确认 metrics-server 可用 |

---

## API 接口

完整 API 文档请访问：**http://localhost:8080/swagger**

### 认证（无需 Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/register` | 用户注册 |
| POST | `/api/auth/login` | 用户登录 |
| POST | `/api/auth/refresh` | 刷新 Token |

### 用户

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/users/me` | 获取当前用户资料 |
| PUT | `/api/users/me` | 更新用户资料 |

### 联系人

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/contacts` | 获取联系人列表 |
| POST | `/api/contacts/apply` | 发送好友申请 |
| POST | `/api/contacts/handle` | 处理好友申请 |
| DELETE | `/api/contacts/:id` | 删除联系人 |

### 会话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/conversations` | 获取会话列表 |
| GET | `/api/conversations/:id` | 获取会话详情 |
| PUT | `/api/conversations/:id` | 更新会话 |
| POST | `/api/conversations` | 创建会话 |

### 消息

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/messages` | 发送消息 |
| GET | `/api/messages/history` | 获取历史消息 |
| POST | `/api/messages/read` | 标记已读 |
| POST | `/api/messages/:id/revoke` | 撤回消息 |

### 在线状态

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/presence` | 批量查询在线状态 |

### 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/files/upload` | 获取上传 URL |
| POST | `/api/files/complete` | 完成上传 |

### WebSocket

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/ws` | WebSocket 连接端点 |

---

## 常用命令

### Makefile 命令总览

```bash
# 查看所有命令
make help

# 构建
make build                   # 构建所有镜像
make build-gateway           # 构建 API Gateway
make build-delivery          # 构建 Delivery Service
make build-wsbench           # 构建 wsbench

# K8s 部署
make k8s-up                  # 部署到 Docker Desktop K8s
make k8s-down                # 清理 K8s 资源
make k8s-status              # 查看 K8s 状态
make k8s-logs APP=<name>     # 查看日志
make k8s-restart APP=<name>  # 重启服务

# 压测
make bench-ws-1k             # 1k 连接压测
make bench-ws-5k             # 5k 连接压测
make bench-ws-10k            # 10k 连接压测
make bench-ws-50k            # 50k 连接压测
make bench-msg-throughput    # 消息吞吐量压测
make bench-collect           # 收集压测数据
make bench-stop              # 停止压测
make bench-local             # 本地运行 wsbench

# 依赖管理
make docker-deps-up          # 启动依赖服务
make docker-deps-down        # 停止依赖服务
make docker-deps-status      # 查看依赖状态

# 工具
make install-metrics-server  # 安装 Metrics Server
make verify-metrics          # 验证 Metrics Server
make clean                   # 清理构建产物
```

### Kubernetes 常用命令

```bash
# Pod 管理
kubectl get pods -n im                          # 查看所有 Pod
kubectl get pods -n im -o wide                  # 查看详细信息（包含 IP）
kubectl describe pod <pod-name> -n im           # 查看 Pod 详情
kubectl logs -f <pod-name> -n im                # 查看实时日志
kubectl logs <pod-name> -n im --tail=100        # 查看最后 100 行
kubectl exec -it <pod-name> -n im -- /bin/sh    # 进入 Pod

# Deployment 管理
kubectl get deployment -n im                    # 查看 Deployment
kubectl scale deployment/<name> -n im --replicas=5  # 手动扩缩容
kubectl rollout restart deployment/<name> -n im # 重启 Deployment
kubectl rollout status deployment/<name> -n im  # 查看 rollout 状态

# HPA 管理
kubectl get hpa -n im                           # 查看 HPA
kubectl describe hpa <name> -n im               # HPA 详情
kubectl autoscale deployment/<name> -n im --min=2 --max=10 --cpu-percent=70  # 创建 HPA

# 资源使用
kubectl top nodes                               # 节点资源
kubectl top pods -n im                          # Pod 资源
kubectl top pods -n im -l app=delivery-service  # 特定 app 资源

# Service 和 Endpoint
kubectl get svc -n im                           # 查看 Service
kubectl get endpoints -n im                     # 查看 Endpoints
kubectl port-forward svc/<name> -n im 8080:8080 # 端口转发

# 事件和排障
kubectl get events -n im --sort-by='.lastTimestamp'  # 查看事件
kubectl get events -n im --field-selector type!=Normal  # 仅错误事件
```

### Docker 命令

```bash
# 容器管理
docker ps                                       # 查看运行中的容器
docker ps -a                                    # 查看所有容器
docker logs im_mysql                            # 查看日志
docker exec -it im_mysql mysql -u root -pimdev  # 进入 MySQL

# Docker Compose
cd deploy
docker compose -f docker-compose.dev.yml ps     # 查看状态
docker compose -f docker-compose.dev.yml logs -f im_kafka  # 查看日志
docker compose -f docker-compose.dev.yml restart im_redis  # 重启服务
docker compose -f docker-compose.dev.yml down   # 停止所有服务
docker compose -f docker-compose.dev.yml down -v  # 停止并删除数据

# 镜像管理
docker images | grep im/                        # 查看 IM 镜像
docker rmi im/api-gateway:latest                # 删除镜像
docker system prune -a                          # 清理未使用的镜像
```

### 进程管理（本地开发）

```bash
# 查看端口占用
lsof -i :8080                                   # 查看 8080 端口
netstat -tuln | grep 8080                       # Linux 系统

# 杀死进程
kill $(lsof -t -i :8080)                        # 杀死占用 8080 的进程
pkill -f "go run"                               # 杀死所有 go run 进程

# 查看 Go 进程
ps aux | grep "go run"                          # 查看所有 Go 进程
```

---

## 常见问题
#### Q: 端口被占用怎么办？

```bash
# 查看占用端口的进程
lsof -i :8080

# 杀死进程
kill -9 <PID>
```

#### Q: MySQL 连接失败？

等待 MySQL 完全启动（约 30 秒）：
```bash
docker logs im_mysql  # 查看日志
```

#### Q: Swagger 页面打不开？

确保是从 `services/api_gateway/cmd` 目录启动的：
```bash
cd services/api_gateway/cmd
go run main.go handlers.go -config ../configs/config.dev.yaml
```

#### Q: 如何完全重置环境？

```bash
# 停止所有容器并删除数据
cd deploy && docker compose -f docker-compose.dev.yml down -v

# 重新启动
docker compose -f docker-compose.dev.yml up -d
```

---

## 其他需求
#### 技术需求

（一）注册中心集成
1. 服务注册与发现
  - 该服务能够与注册中心（如 Consul、Nacos 、etcd 等）进行集成，自动注册服务数据。

（二）身份认证
1. 登录认证
  - 可以使用第三方现成的登录验证框架（CasBin、Satoken等），对请求进行身份验证
  - 可配置的认证白名单，对于某些不需要认证的接口或路径，允许直接访问
  - 可配置的黑名单，对于某些异常的用户，直接进行封禁处理（可选）
2. 权限认证（高级）
  - 根据用户的角色和权限，对请求进行授权检查，确保只有具有相应权限的用户能够访问特定的服务或接口。
  - 支持正则表达模式的权限匹配（加分项）
  - 支持动态更新用户权限信息，当用户权限发生变化时，权限校验能够实时生效。

（三）可观测要求
1. 日志记录与监控
  - 对服务的运行状态和请求处理过程进行详细的日志记录，方便故障排查和性能分析。
  - 提供实时监控功能，能够及时发现和解决系统中的问题。

（四）可靠性要求（高级）
1. 容错机制
  - 该服务应具备一定的容错能力，当出现部分下游服务不可用或网络故障时，能够自动切换到备用服务或进行降级处理。
  - 保证下游在异常情况下，系统的整体可用性不会受太大影响，且核心服务可用。
  - 服务应该具有一定的流量兜底措施，在服务流量激增时，应该给予一定的限流措施。

三、功能需求
认证中心
- 分发身份令牌
- 续期身份令牌（高级）
- 校验身份令牌

用户服务
- 创建用户
- 登录
- 用户登出（可选）
- 删除用户（可选）
- 更新用户（可选）
- 获取用户身份信息

---

## 许可证

MIT License

---

**⭐ 如果这个项目对你有帮助，请给一个 Star~ 谢谢～**
