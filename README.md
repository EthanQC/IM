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
│          统一入口 / JWT 认证 / 限流 / 路由转发                      │
└─────────┬───────────────────────────────────────────────────────┘
          │
          ├──────────────────────────────────────────┐
          │                                          │
          ↓ gRPC                                     ↓ WebSocket
┌──────────────────────┐                  ┌──────────────────────┐
│  微服务层 (9080+)     │                  │  Delivery Service    │
│                      │                  │  (WebSocket 网关)     │
│ • Identity Service   │                  │  • 长连接管理          │
│ • Conversation Svc   │                  │  • 消息投递           │
│ • Message Service    │                  │  • 在线路由           │
│ • Presence Service   │                  │  • WebRTC 信令        │
│ • File Service       │                  └──────────┬───────────┘
└──────────┬───────────┘                             │
           │                                         │
           └─────────────────┬───────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│                      数据与消息层                                 │
│                                                                 │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐             │
│  │  MySQL  │  │  Redis  │  │  Kafka  │  │  MinIO  │             │
│  │  :3306  │  │  :6379  │  │ :29092  │  │  :9000  │             │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘             │
│   主存储       缓存/会话     消息队列      对象存储                  │
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

## 性能指标 - 实测数据

### 测试环境

| 配置项 | 值 | 说明 |
|--------|-----|------|
| **环境** | Docker Desktop + Kubernetes | 单节点集群 |
| **操作系统** | macOS | Docker Desktop 内置 K8s |
| **压测工具** | wsbench (本地执行) | Go 实现，支持 connect-only/messaging 模式 |
| **Delivery Service** | 4-8 Pod (HPA) | 根据负载自动扩缩容 |
| **压测日期** | 2026-01-22 | 实际测试数据 |

### WebSocket 连接压测结果

以下是在 Docker Desktop 单节点 K8s 环境下的**实测数据**：

| 连接数 | 成功连接 | 成功率 | P50 延迟 | P99 延迟 | 心跳响应率 | 状态 |
|--------|----------|--------|----------|----------|------------|------|
| **1,000** | 1,000 | 100.00% | 1.85ms | 7.46ms | 100.00% | ✅ 完美 |
| **10,000** | 9,533 | 95.33% | 1.61ms | 11.69ms | 99.53% | ✅ 稳定 |
| **30,000** | 5,397 | 17.99% | 2.09ms | 341.49ms | 100.00% | ❌ 瓶颈 |
| **50,000** | 10,533 | 21.07% | 1.43ms | 28.82ms | 100.00% | ❌ 瓶颈 |

### 结论与分析

**Docker Desktop 环境能力边界**：
- ✅ **1K 连接**：完美稳定，100% 成功率，延迟极低
- ✅ **10K 连接**：95%+ 成功率，适合日常开发验证
- ❌ **30K+ 连接**：Docker Desktop 网络栈瓶颈，成功率下降

**瓶颈归因**：

| 瓶颈项 | 说明 | 解决方案 |
|--------|------|----------|
| **Docker Desktop 网络栈** | 虚拟化网络层在高连接数下性能受限 | 迁移到 Linux 服务器 |
| **单机文件描述符** | 即使调整 ulimit，Docker 层仍有限制 | 使用真实 K8s 集群 |
| **端口范围** | 客户端临时端口可能耗尽 | 调整 `ip_local_port_range` |

**预期生产环境性能**：
- 真实 K8s 集群（3 节点）+ 系统调优 → **50K+ 稳定连接**
- 单节点 Linux 服务器（16核32G）+ 参数调优 → **30K+ 连接**

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

本项目提供简单易用的压测命令，可快速验证 WebSocket 连接能力。

### 快速运行

```bash
# 确保 K8s 环境已部署
make k8s-up

# 1K 连接（快速验证）
make bench-ws-1k

# 10K 连接（稳定测试）
make bench-ws-10k

# 30K/50K 连接（Docker Desktop 会受限）
make bench-ws-30k
make bench-ws-50k
```

### 压测工具

**wsbench** - 自研 Go WebSocket 压测工具

| 特性 | 说明 |
|------|------|
| **模式** | connect-only（仅连接）、messaging（消息吞吐） |
| **爬坡** | 渐进式建立连接，避免雪崩 |
| **心跳** | 自动 Ping/Pong 保活 |
| **指标** | 连接成功率、延迟百分位、心跳响应率 |

**源码位置**：[bench/wsbench/main.go](bench/wsbench/main.go)

### 输出示例

```
==================== 压测结果 ====================

--- 连接统计 ---
尝试连接数:     10000
成功连接数:     9533
失败连接数:     467
连接成功率:     95.33%

--- 连接延迟 (ms) ---
P50:    1.61
P99:    11.69

--- 心跳统计 ---
Pong 响应率:    99.53%

=================================================
```

### 数据收集

```bash
# 自动收集压测数据
make bench-collect

# 数据保存到 bench/results/<timestamp>/
```

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
