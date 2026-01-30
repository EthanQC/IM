# Instant Messaging System

中文 | [English](README_EN.md)

基于**微服务架构**的生产级即时通讯系统，采用 **DDD（领域驱动设计）+ 六边形架构**，支持 **100k+ 并发 WebSocket 连接**

[![MIT License](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![gRPC](https://img.shields.io/badge/gRPC-Protocol-244c5a?style=flat&logo=grpc)](https://grpc.io)
[![Kafka](https://img.shields.io/badge/Kafka-KRaft-231F20?style=flat&logo=apachekafka)](https://kafka.apache.org)
[![Redis](https://img.shields.io/badge/Redis-7.2-DC382D?style=flat&logo=redis)](https://redis.io)
[![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql&logoColor=white)](https://www.mysql.com)
[![Vue.js](https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat&logo=vue.js)](https://vuejs.org)

---

## 目录

- [功能特性](#功能特性)
- [技术架构](#技术架构)
- [快速开始](#快速开始)
  - [本地开发](#本地开发)
  - [部署上云](#部署上云)
- [高并发压测](#高并发压测)
  - [压测总原则](#压测总原则)
  - [环境准备与系统调优](#环境准备与系统调优)
  - [场景1：连接层压测](#场景1连接层压测)
  - [场景2：消息链路压测](#场景2消息链路压测)
  - [场景3：在线状态与重连](#场景3在线状态与重连)
  - [场景4：系统稳定性测试](#场景4系统稳定性测试)
  - [压测结果汇总](#压测结果汇总)
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
- **高并发** - 单节点支持 30k+ 稳定连接，多机集群支持 100k+
- **自动扩缩容** - Kubernetes HPA 基于 CPU/内存自动调整
- **可观测** - Prometheus + pprof 全链路监控
- **安全认证** - JWT Token 认证，支持刷新
- **消息可靠性** - Kafka 消息队列 + 死信队列 + ACK 机制
- **数据一致性** - Redis Lua 脚本原子操作
- **分布式** - 支持多实例部署，全局在线路由

---

## 技术架构

### 技术栈

| 分类 | 技术选型 | 版本/说明 |
|------|---------|----------|
| **编程语言** | Go | 1.25 |
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

---

## 快速开始

### 前置要求

| 软件 | 版本要求 | 验证命令 |
|------|---------|----------|
| Go | 1.25 | `go version` |
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

#### 2. 下载 Go 依赖

```bash
# 使用 Go workspace 模式，一次性下载所有模块依赖
go work sync

# 或者分别进入各服务目录下载（首次运行可能需要几分钟）
for svc in api_gateway identity_service conversation_service message_service delivery_service presence_service file_service; do
  echo ">>> Downloading $svc dependencies..."
  (cd services/$svc && go mod download)
done

# 下载压测工具依赖
cd bench/wsbench && go mod download && cd ../..
```

#### 3. 启动依赖服务

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

#### 4. 初始化数据库

```bash
# 连接 MySQL 容器并执行 schema.sql
docker exec -i im_mysql mysql -uroot -pimdev < deploy/sql/schema.sql
```

#### 5. 初始化配置文件

各服务的配置文件需要从 `.example` 模板复制：

```bash
# 方式一：一键初始化（推荐）
bash scripts/init-configs.sh

# 方式二：手动复制（如需自定义配置）
for svc in api_gateway identity_service conversation_service message_service delivery_service presence_service file_service; do
  cp services/$svc/configs/config.dev.yaml.example services/$svc/configs/config.dev.yaml
done

# 批量替换密码（将 your_password 替换为 Docker Compose 默认密码 imdev）
# macOS:
find services -name "config.dev.yaml" -exec sed -i '' 's/your_password/imdev/g' {} \;
# Linux:
find services -name "config.dev.yaml" -exec sed -i 's/your_password/imdev/g' {} \;
```

**配置文件说明：**

| 文件 | 说明 | 是否提交 Git |
|------|------|-------------|
| `config.dev.yaml.example` | 配置模板（占位符） | ✅ 提交 |
| `config.dev.yaml` | 实际配置（含真实密码） | ❌ 不提交 |
| `config.prod.yaml.example` | 生产环境模板 | ✅ 提交 |
| `config.prod.yaml` | 生产环境配置 | ❌ 不提交 |

**需要修改的配置项（仅当默认值不适用时）：**

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `mysql.dsn` | `root:imdev@tcp(127.0.0.1:3306)/im_db` | MySQL 连接串 |
| `redis.addr` | `127.0.0.1:6379` | Redis 地址 |
| `kafka.brokers` | `127.0.0.1:29092` | Kafka 地址 |
| `jwt.secret` | 默认 32 字符 | JWT 密钥（生产环境必须修改） |

#### 6. 启动微服务

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

#### 7. 验证部署

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

### 部署上云 / Docker 一键部署

本项目支持通过 Docker Compose 一键部署全部服务，适用于：
- 本地压测环境
- 云服务器生产部署

#### 服务器要求（云部署）

| 配置 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 2核 | 4核+ |
| 内存 | 4GB | 8GB+ |
| 硬盘 | 40GB SSD | 100GB SSD |
| 系统 | Ubuntu 22.04 / macOS | Linux 推荐 |

#### Docker 一键部署步骤

```bash
# 1. 克隆项目
git clone https://github.com/EthanQC/IM.git
cd IM

# 2. 系统调优（高并发必须）
# macOS:
sudo bash scripts/tune-macos.sh
# Linux:
sudo bash scripts/tune-server.sh

# 3. 启动完整服务栈（首次需要构建镜像，约 3-5 分钟）
cd deploy
docker compose -f docker-compose.bench.yml up -d --build

# 4. 查看服务状态（等待所有容器 healthy/running）
docker compose -f docker-compose.bench.yml ps

# 5. 初始化数据库
docker exec -i im_mysql mysql -uroot -pimdev < sql/schema.sql

# 6. 验证部署
curl http://localhost:8080/healthz
curl http://localhost:8084/stats
```

**服务列表：**

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| MySQL | im_mysql | 3306 | root / imdev |
| Redis | im_redis | 6379 | 无密码 |
| Kafka | im_kafka | 29092 | KRaft 模式 |
| MinIO | im_minio | 9000/9001 | admin / admin123 |
| API Gateway | im_gateway | **8080** | HTTP API |
| Delivery Service | im_delivery | **8084** | WebSocket |

#### 停止/重启服务

```bash
# 停止所有服务
docker compose -f docker-compose.bench.yml down

# 停止并删除数据卷（完全重置）
docker compose -f docker-compose.bench.yml down -v

# 重启单个服务
docker compose -f docker-compose.bench.yml restart delivery-service

# 查看日志
docker compose -f docker-compose.bench.yml logs -f delivery-service
```

#### 防火墙配置（云服务器）

```bash
# 开放必要端口
sudo ufw allow 8080/tcp  # API Gateway
sudo ufw allow 8084/tcp  # WebSocket
sudo ufw enable
```

#### 域名与 HTTPS（可选）

推荐使用 Nginx 反向代理 + Let's Encrypt：

```bash
# 安装 Nginx 和 Certbot
sudo apt install nginx certbot python3-certbot-nginx

# 配置反向代理
sudo vim /etc/nginx/sites-available/im
# 添加代理配置指向 localhost:8080 和 localhost:8084

# 申请证书
sudo certbot --nginx -d your-domain.com
```

---

## 高并发压测

本节提供完整的压测指南，使用 Docker Compose 部署服务，配合 wsbench 工具进行压测。

### 压测总原则

> ⚠️ **重要**：在开始前务必阅读

1. **使用 Docker Compose 部署**：所有服务通过容器运行，避免本地环境差异
2. **分离测试**：连接压测和消息压测分开跑，否则无法定位瓶颈
3. **多轮测试**：每个场景至少跑 3 轮（冷启动、热身后、调参后）
4. **完整记录**：每个场景记录 规模参数 + 成功率 + p95/p99 + 资源曲线 + 队列积压
5. **区分瓶颈**：区分"服务端瓶颈"和"压测端瓶颈"

#### 单机连接数限制

单台机器的出站连接数受限于本地端口范围（通常 1024-65535），即约 **64,000** 个连接。

如需达到 **100k+** 连接，需要：
- 使用 2+ 台压测机器
- 或在单机配置多个 IP 地址

#### 硬件拓扑

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              局域网 (1Gbps+)                                 │
└────────────────┬─────────────────────┬─────────────────────┬────────────────┘
                 │                     │                     │
                 ▼                     ▼                     ▼
┌─────────────────────────┐ ┌─────────────────────────┐ ┌─────────────────────────┐
│   Node-A (服务节点)      │ │   Node-B (压测节点1)     │ │   Node-C (压测节点2)     │
│   Mac Mini / Linux      │ │   Linux / WSL2          │ │   Linux / WSL2          │
│                         │ │                         │ │                         │
│ ┌─────────────────────┐ │ │ ┌─────────────────────┐ │ │ ┌─────────────────────┐ │
│ │ Docker Compose      │ │ │ │ wsbench 压测工具    │ │ │ │ wsbench 压测工具    │ │
│ │ 全套微服务 + 依赖    │ │ │ │ 目标: 50k 连接      │ │ │ │ 目标: 50k 连接      │ │
│ └─────────────────────┘ │ │ └─────────────────────┘ │ │ └─────────────────────┘ │
│   IP: 192.168.x.x       │ └─────────────────────────┘ └─────────────────────────┘
└─────────────────────────┘
```

---

### 环境准备与系统调优

> ⚠️ **系统调优是高并发的前提**，不调优会遇到假瓶颈（FD 耗尽、端口耗尽、TIME_WAIT 堆积）

#### Step 1: 服务节点准备 (Docker Compose 部署)

```bash
# 1. 克隆项目
git clone https://github.com/EthanQC/IM.git && cd IM

# 2. 系统调优（必须！100k 连接需要调优）
# macOS:
sudo bash scripts/tune-macos.sh
# Linux:
sudo bash scripts/tune-server.sh

# 3. 验证调优结果
ulimit -n                # 应显示 >= 1000000
sysctl kern.ipc.somaxconn 2>/dev/null || sysctl net.core.somaxconn  # 应显示 >= 65535

# 4. 启动完整服务栈（Docker Compose）
cd deploy
docker compose -f docker-compose.bench.yml up -d --build

# 5. 等待所有服务 healthy（约 1-2 分钟）
docker compose -f docker-compose.bench.yml ps

# 6. 初始化数据库（如果还没有）
docker exec -i im_mysql mysql -uroot -pimdev < sql/schema.sql

# 7. 获取 IP（告知压测节点）
# macOS:
ipconfig getifaddr en0 || ipconfig getifaddr en1
# Linux:
hostname -I | awk '{print $1}'

# 8. 验证服务
curl http://localhost:8080/healthz
curl http://localhost:8084/stats  # WebSocket 服务状态
```

#### Step 2: 压测节点准备 (WSL2 / Linux / macOS)

**方法一：WSL2 环境配置**

在 Windows PowerShell 中执行（`Win+X` → PowerShell）：

```powershell
# 创建 .wslconfig
@"
[wsl2]
memory=28GB
processors=12
swap=16GB
localhostForwarding=true
"@ | Out-File -FilePath "$env:USERPROFILE\.wslconfig" -Encoding utf8

# 重启 WSL
wsl --shutdown
```

**进入 WSL2 后执行：**

```bash
# 1. 克隆项目
git clone https://github.com/EthanQC/IM.git && cd IM

# 2. 系统调优（必须！）
sudo bash scripts/tune-bench-client.sh

# 3. 验证调优（在同一终端直接执行）
ulimit -n                           # 应显示 >= 500000
sysctl net.core.somaxconn           # 应显示 65535
sysctl net.ipv4.ip_local_port_range # 应显示 1024 65535

# 4. 编译压测工具
cd bench/wsbench && go build -o wsbench .

# 5. 查看压测参数
./wsbench --help
```

**方法二：macOS 压测机器**

```bash
# 1. 克隆项目
git clone https://github.com/EthanQC/IM.git && cd IM

# 2. 系统调优
sudo bash scripts/tune-bench-client.sh

# 3. 验证
ulimit -n  # 应显示 >= 500000

# 4. 编译压测工具
cd bench/wsbench && go build -o wsbench .
```

#### Step 3: 测试连通性

```bash
# 替换为 Node-A 的实际 IP
SERVER_IP="192.168.1.100"

ping $SERVER_IP
curl http://$SERVER_IP:8080/healthz
curl http://$SERVER_IP:8084/stats

# 快速验证 WebSocket 连接
./wsbench --target=ws://$SERVER_IP:8084/ws --conns=100 --duration=30s --ramp=5s
```

#### wsbench 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--target` | ws://localhost:8084/ws | WebSocket 服务器地址 |
| `--conns` | 1000 | 目标连接数 |
| `--duration` | 5m | 压测持续时间 |
| `--ramp` | 1m | 爬坡时间（建立连接的时间跨度） |
| `--ping-interval` | 30s | 心跳间隔 |
| `--handshake-timeout` | 30s | 握手超时（高并发时需要更长） |
| `--max-cps` | 500 | **每秒最大连接数（限速，避免瞬时压力过大）** |
| `--retry` | 3 | **连接失败重试次数** |
| `--retry-delay` | 1s | **重试延迟（指数退避）** |
| `--read-timeout` | 2m | **读超时（防止连接假死）** |
| `--write-timeout` | 10s | 写超时 |
| `--read-buffer` | 8192 | 读缓冲区大小 |
| `--write-buffer` | 8192 | 写缓冲区大小 |
| `--mode` | connect-only | 模式：connect-only / messaging |
| `--msg-rate` | 10 | 每连接每分钟消息数（messaging 模式） |
| `--output` | text | 输出格式：text / json / csv |

---

### 场景1：连接层压测

> 目的：验证 Go 高并发能力，测试 Delivery Service 单机能撑多少稳定连接

#### 1.1 Connect-Only 极限连接数

**测试目标**：只建立 WebSocket 连接，不发消息，验证稳定维持能力

> ⚠️ **重要参数说明**：
> - `--max-cps=300`：每秒最大建立 300 个连接，避免服务端处理不过来导致连接被拒绝
> - `--ping-interval=60s`：与服务端心跳间隔一致
> - `--read-timeout=180s`：与服务端 pongWait 一致

```bash
# ===== 在压测节点执行 =====
cd IM/bench/wsbench

# 设置 ulimit（每个终端都要执行）
ulimit -n 500000

# 预热测试（验证环境正常）
./wsbench --target=ws://192.168.1.100:8084/ws --conns=1000 --duration=1m --ramp=10s

# 阶梯压测（找到极限）
# 10k 连接 - 约 33 秒建立完成
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=5m --ramp=1m --max-cps=300 --ping-interval=60s --read-timeout=180s

# 30k 连接 - 约 100 秒建立完成
./wsbench --target=ws://192.168.1.100:8084/ws --conns=30000 --duration=10m --ramp=2m --max-cps=300 --ping-interval=60s --read-timeout=180s

# 50k 连接 - 约 167 秒建立完成（单机极限）
./wsbench --target=ws://192.168.1.100:8084/ws --conns=50000 --duration=15m --ramp=5m --max-cps=300 --ping-interval=60s --read-timeout=180s

# ===== 双机联合 100k 目标 =====
# 使用一键脚本（推荐）：
# Node-B 执行:
bash scripts/run-50k-bench.sh 192.168.1.100:8084

# Node-C 同时执行:
bash scripts/run-50k-bench.sh 192.168.1.100:8084

# 或手动执行（带完整参数）：
# Node-B:
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=50000 \
    --duration=30m \
    --ramp=5m \
    --ping-interval=60s \
    --handshake-timeout=30s \
    --max-cps=300 \
    --retry=3 \
    --retry-delay=2s \
    --read-timeout=180s

# Node-C 同时执行相同命令
```

**实时监控（在服务节点执行）**：

```bash
# 使用监控脚本
bash scripts/monitor-bench.sh localhost:8084
```

# 终端2: Goroutine 数量
watch -n 1 'curl -s http://localhost:8084/metrics | grep go_goroutines'

# 终端3: 内存使用
watch -n 1 'curl -s http://localhost:8084/metrics | grep go_memstats_heap_inuse_bytes'

# 终端4: 网络连接状态
watch -n 2 'netstat -an | grep 8084 | grep ESTABLISHED | wc -l'
```

**需要记录的指标**：

| 指标 | 说明 | 目标 |
|------|------|------|
| 成功连接数峰值 | 最大同时在线 | 100k+ |
| 稳定维持时长 | 无大规模断连 | 30min+ |
| 建连失败率 | 429/5xx/超时/reset | < 1% |
| 建连延迟 p50/p95/p99 | 握手耗时 | < 100ms |
| 断开连接数 | 压测期间意外断开 | < 1% |
| 服务端 Goroutine | curl metrics | ~2x 连接数 |
| 服务端内存 | heap_inuse_bytes | 记录曲线 |
| GC Pause | pprof 或日志 | < 10ms |

**常见问题排查**：

| 错误类型 | 可能原因 | 解决方案 |
|----------|----------|----------|
| `conn_refused` | 服务端未启动或端口未开放 | 检查服务状态和防火墙 |
| `timeout` | 握手超时 | 增加 `--handshake-timeout` |
| `fd_exhausted` | 文件描述符耗尽 | 运行 `tune-bench-client.sh` |
| `conn_reset` | 服务端资源不足或被限流 | 降低 `--max-cps` |
| `eof` | 连接被服务端关闭 | 检查服务端日志 |

**pprof 分析（压测进行时）**：

```bash
# 需要先安装 graphviz: brew install graphviz (macOS) 或 apt install graphviz (Linux)

# 内存分析
go tool pprof -http=:8000 http://localhost:8084/debug/pprof/heap

# Goroutine 分析
go tool pprof -http=:8001 http://localhost:8084/debug/pprof/goroutine

# CPU 分析（30秒采样）
go tool pprof -http=:8002 http://localhost:8084/debug/pprof/profile?seconds=30
```

#### 1.2 建连速率上限 (Ramp-up)

**测试目标**：测试每秒能新建多少连接

```bash
# 注意：压测工具默认限制 500 conn/s，可通过 --max-cps 调整

# 测试不同连接速率
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=2m --ramp=20s --max-cps=500   # 500 conn/s
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=2m --ramp=10s --max-cps=1000  # 1000 conn/s
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=2m --ramp=5s --max-cps=2000   # 2000 conn/s
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=2m --ramp=2s --max-cps=5000   # 5000 conn/s
```

**记录**：不同 `--max-cps` 下的失败率拐点

#### 1.3 心跳与空闲连接稳定性

**测试目标**：长连接稳定保活，不是"连上就算"

```bash
# 维持 50k 连接 30 分钟，观察心跳
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=50000 \
    --duration=30m \
    --ramp=5m \
    --ping-interval=45s \
    --read-timeout=120s
```

**需要记录**：
- Ping/Pong 成功率（应接近 100%）
- 超时断开数
- 最终连接数 vs 初始连接数

#### 1.4 广播下行压力

**测试目标**：服务端向所有连接推送小消息的能力

```bash
# 10k 连接，每连接每分钟 1 条消息（messaging 模式）
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=10000 \
    --duration=5m \
    --ramp=1m \
    --mode=messaging \
    --msg-rate=1 \
    --payload-size=100

# 更高压力：每连接每分钟 5 条消息
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=10000 \
    --duration=5m \
    --ramp=1m \
    --mode=messaging \
    --msg-rate=5 \
    --payload-size=100
```

---

### 场景2：消息链路压测

> 目的：测试 IM 核心能力 —— 消息发送与接收

#### 2.1 单聊吞吐与端到端延迟

**测试目标**：消息从发送到对端收到的完整链路

```bash
# 基础：5k 连接，每连接每分钟 1 条消息
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=5000 \
    --duration=5m \
    --ramp=1m \
    --mode=messaging \
    --msg-rate=1 \
    --payload-size=100

# 中等：10k 连接，每连接每分钟 5 条消息
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=10000 \
    --duration=5m \
    --ramp=2m \
    --mode=messaging \
    --msg-rate=5 \
    --payload-size=100

# 高压：20k 连接，每连接每分钟 10 条消息
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=20000 \
    --duration=5m \
    --ramp=3m \
    --mode=messaging \
    --msg-rate=10 \
    --payload-size=100
```

**需要记录的指标**：

| 指标 | 说明 | 目标 |
|------|------|------|
| msg/s 成功发送 | 发送吞吐 | 100k+ |
| msg/s 成功接收 | 投递吞吐 | 接近发送 |
| 端到端 RTT p50/p95/p99 | 消息延迟 | < 50ms |
| 丢消息率 | seq 校验 | 0% |
| 重复率 | msg_id 校验 | 0% |
| Kafka lag | consumer 积压 | < 1000 |
| DB 写入延迟 | 慢查询日志 | < 10ms |

**Kafka 监控**：

```bash
# 查看 consumer lag（需要 kafka 客户端）
docker exec im_kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group im-delivery
```

#### 2.2 小群聊 Fanout

**测试目标**：群消息写扩散能力

```bash
# 需要先通过 API 创建群，然后压测群消息
# 这里假设群 ID 为 "group_100"，有 100 个成员

# 模拟群聊：100 人群，10 人同时发言
# 需要定制 wsbench 或使用脚本调用 API
```

#### 2.3 顺序一致性测试

**测试目标**：证明 Kafka 分区键保证局部有序

```bash
# 同一会话高并发发消息，接收端校验 seq 单调递增
# 需要 wsbench 增加 seq 校验功能，或手动测试：
# 1. 多个客户端同时向同一会话发消息
# 2. 接收端记录所有消息的 seq
# 3. 验证 seq 严格递增，无乱序无缺失
```

---

### 场景3：在线状态与重连

#### 3.1 重连风暴测试

**测试目标**：模拟网络抖动，全员重连时的恢复能力

```bash
# 1. 建立 30k 稳定连接
./wsbench -target=ws://192.168.1.100:8084/ws -conns=30000 -duration=10m -ramp=2m

# 2. 压测进行中，模拟断网（在压测机执行）
#    方法：暂停 wsbench 进程 10 秒
kill -STOP $(pgrep wsbench)
sleep 10
kill -CONT $(pgrep wsbench)

# 观察 wsbench 的重连行为和服务端的恢复
```

**需要记录**：
- 重连成功率
- 恢复到稳态耗时
- 重连期间消息丢失率
- 服务端 CPU 峰值

#### 3.2 离线补拉测试

**测试目标**：验证 Last_Ack_Seq 增量拉取

```bash
# 手动测试流程：
# 1. 用户 A 在线，用户 B 发送 100 条消息
# 2. A 断线 1 分钟
# 3. 期间 B 继续发送 50 条消息
# 4. A 重连，触发补拉
# 5. 验证 A 收到完整 50 条，无缺失无重复

# 需要通过 API 或定制客户端测试
```

#### 3.3 Presence 热点压力

**测试目标**：在线状态高频写入

```bash
# 大量用户频繁上下线
# 可通过快速建连-断开-建连模拟
./wsbench -target=ws://192.168.1.100:8084/ws -conns=5000 -duration=30s -ramp=5s
# 脚本循环执行，观察 Redis 写入压力
```

---

### 场景4：系统稳定性测试

#### 4.1 长稳 Soak 测试

**测试目标**：证明无内存泄漏、无 goroutine 爆炸

```bash
# 中等负载长时间运行
# 30k 连接，每连接每分钟 1 条消息，持续 4 小时
./wsbench \
    --target=ws://192.168.1.100:8084/ws \
    --conns=30000 \
    --duration=4h \
    --ramp=10m \
    --mode=messaging \
    --msg-rate=1 \
    --ping-interval=45s \
    --read-timeout=120s

# 同时持续监控服务端（新开终端）
while true; do
  echo "$(date): conns=$(curl -s http://localhost:8084/stats | grep -o '"total_connections":[0-9]*' | cut -d':' -f2) goroutines=$(curl -s http://localhost:8084/metrics | grep '^go_goroutines ' | awk '{print $2}') heap=$(curl -s http://localhost:8084/metrics | grep '^go_memstats_heap_inuse_bytes ' | awk '{print $2}')"
  sleep 60
done | tee soak_metrics.log
```

**需要记录**：
- 内存曲线（应稳定，不应持续上升）
- goroutine 数量（应稳定）
- GC 频率和耗时
- 错误率随时间变化（应稳定）

#### 4.2 背压与过载保护

**测试目标**：系统到瓶颈时优雅降级，不雪崩

```bash
# 逐步提高消息速率，直到触发限流
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=3m --ramp=1m --mode=messaging --msg-rate=10
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=3m --ramp=1m --mode=messaging --msg-rate=20
./wsbench --target=ws://192.168.1.100:8084/ws --conns=10000 --duration=3m --ramp=1m --mode=messaging --msg-rate=50
```

**观察**：
- 延迟是"缓慢上升"还是"突然爆炸"
- 是否有 429 限流响应
- 错误码分布

#### 4.3 故障注入测试

**测试目标**：验证可靠性设计

```bash
# 1. Kafka 短暂不可用
docker stop im_kafka && sleep 30 && docker start im_kafka
# 观察：消息是否丢失、恢复时间、积压消化速度

# 2. Redis 重启
docker restart im_redis
# 观察：在线状态是否恢复、会话数据是否正常

# 3. MySQL 慢查询模拟
# 在 MySQL 执行: SET GLOBAL slow_query_log = 1; SET GLOBAL long_query_time = 0.001;
# 观察日志中的慢查询
```

---

### 压测结果汇总

完成以上测试后，你应该能产出以下数据（可直接写入简历）：

#### 核心指标模板

| 指标 | 测试值 | 目标 | 状态 |
|------|--------|------|------|
| 最大稳定 WS 连接数 | _____ | 100k+ | [ ] |
| 连接稳定维持时长 | _____min | 30min+ | [ ] |
| 建连速率上限 | _____conn/s | 5k+/s | [ ] |
| 单聊吞吐 | _____msg/s | 100k+ | [ ] |
| 端到端延迟 p95 | _____ms | < 50ms | [ ] |
| 端到端延迟 p99 | _____ms | < 100ms | [ ] |
| 消息丢失率 | _____% | 0% | [ ] |
| 重连风暴恢复时间 | _____s | < 30s | [ ] |
| 离线补拉正确率 | _____% | 100% | [ ] |
| 长稳 4h 内存稳定 | 是/否 | 是 | [ ] |

#### 简历可写指标

**保守写法**（基于实测数据）：
> 支持 **30,000+** 并发 WebSocket 连接，消息吞吐 **100,000 msg/s**，端到端延迟 **< 50ms (P95)**

**进阶写法**（需完整跑完上述测试）：
> 单集群支持 **80,000+** 并发长连接，消息吞吐 **300,000 msg/s**，实现重连风暴 30 秒内恢复

**极限写法**（三机满载）：
> 分布式 IM 系统支持 **100,000+** 并发连接，消息吞吐 **500,000+ msg/s**，通过 4 小时 Soak 测试无内存泄漏

---

### 监控与调试命令速查

```bash
# ===== 服务端 (Node-A) =====

# 一键监控脚本（推荐）
bash scripts/monitor-bench.sh localhost:8084

# 手动监控命令
# 实时连接统计
watch -n 1 'curl -s http://localhost:8084/stats'

# Goroutine 数量（每个连接约 2 个）
watch -n 1 'curl -s http://localhost:8084/metrics | grep "^go_goroutines "'

# 内存使用
watch -n 1 'curl -s http://localhost:8084/metrics | grep "^go_memstats_heap_inuse_bytes "'

# 网络连接状态
watch -n 2 'netstat -an | grep 8084 | grep ESTABLISHED | wc -l'

# pprof 分析
go tool pprof http://localhost:8084/debug/pprof/heap
go tool pprof http://localhost:8084/debug/pprof/goroutine
go tool pprof -http=:8000 http://localhost:8084/debug/pprof/profile?seconds=30

# ===== 压测端 (Node-B/C) =====

# 确认 ulimit
ulimit -n  # 应 >= 500000

# 连接状态
ss -s | grep estab
netstat -an | grep ESTABLISHED | wc -l

# 端口使用情况
# Linux:
cat /proc/sys/net/ipv4/ip_local_port_range
# macOS:
sysctl net.inet.ip.portrange.first net.inet.ip.portrange.last

# 查看压测错误分布
./wsbench --target=ws://192.168.1.100:8084/ws --conns=1000 --duration=1m --verbose
```

### 新增脚本说明

| 脚本 | 位置 | 用途 |
|------|------|------|
| `tune-server.sh` | scripts/ | 服务端系统调优（支持 100k+ 连接） |
| `tune-bench-client.sh` | scripts/ | 压测客户端系统调优 |
| `monitor-bench.sh` | scripts/ | 实时监控压测状态 |
| `run-50k-bench.sh` | scripts/ | 一键运行 50k 连接压测 |

---
## 常见问题

### 压测相关问题

#### Q: 压测时大量连接建立失败？

1. **检查系统调优**：
   ```bash
   # 压测机器
   ulimit -n  # 应 >= 500000
   
   # 服务端
   ulimit -n  # 应 >= 1000000
   ```

2. **降低连接速率**：
   ```bash
   # 使用 --max-cps 限制每秒连接数
   ./wsbench --max-cps=300 ...
   ```

3. **增加握手超时**：
   ```bash
   ./wsbench --handshake-timeout=60s ...
   ```

#### Q: 压测时连接中途大量断开？

1. **检查心跳配置**：
   ```bash
   # 使用更长的心跳间隔和读超时
   ./wsbench --ping-interval=45s --read-timeout=120s ...
   ```

2. **检查服务端资源**：
   ```bash
   # 监控 Goroutine 和内存
   curl http://localhost:8084/metrics | grep go_goroutines
   curl http://localhost:8084/metrics | grep go_memstats_heap_inuse_bytes
   ```

3. **查看错误分布**：
   ```bash
   ./wsbench --verbose ...  # 查看详细错误
   ```

#### Q: 每台机器最多能建立多少连接？

受限于本地端口数量，单台机器最多约 **64,000** 个出站连接（端口范围 1024-65535）。

如需超过 64k 连接，需要：
- 配置多个本地 IP 地址
- 或使用多台压测机器

#### Q: 服务端能支持多少连接？

理论上 Go 单机可支持百万级连接，实际受限于：
- 内存：每个连接约 10-20KB
- CPU：心跳处理和消息分发
- 文件描述符：需要 `ulimit -n` 足够大

Mac Mini M4 16GB 实测可稳定支持 **30k+** 连接，通过双压测机器可达 **100k+**。

### 环境配置问题

### Q: 端口被占用怎么办？

```bash
# 查看占用端口的进程
lsof -i :8080

# 杀死进程
kill -9 <PID>
```

### Q: MySQL 连接失败？

等待 MySQL 完全启动（约 30 秒）：
```bash
docker logs im_mysql  # 查看日志
```

### Q: Swagger 页面打不开？

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

## 许可证

MIT License

---

**⭐ 如果这个项目对你有帮助，请给一个 Star~ 谢谢～**
