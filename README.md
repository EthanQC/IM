# Instant Messaging

基于**微服务架构**的即时通讯系统，采用 **DDD（领域驱动设计）+ 六边形架构**

---

## 功能特性

- 单聊 / 群聊
- 联系人管理
- 多种消息类型（文本/图片/文件/音视频）
- 离线消息处理
- 文件共享
- 音视频通话（WebRTC）

---

## 技术栈

* 后端
  * 语言
    * Go 1.25.5
    * Web 框架：Gin
    * ORM：GORM
    * go-redis
  * 服务间通信
    * 同步：gRPC + Protobuf
    * 异步：Kafka（消息队列）
  * 接口定义与代码生成
    * protobuf
  * 数据库
    * MySQL
    * Redis（在线状态/缓存/限流）
  * 日志
    * zap
  * 监测
    * Prometheus + Grafana + Alertmanager + OpenTelemetry
    * 或用夜莺等其他方案
* 文件存储
  * MinIO（本地对象存储，兼容 S3 API）
* 音视频
  * WebRTC
  * 后端做信令、STUN/TURN 配置
* 容器化与部署
  * Docker 与 Docker Compose（本地开发）
  * Kubernetes（生产）
  * CI/CD：Github Actions
* 前端
  * Vue3 + Vite + TS

---

## 架构总览
本项目采用前后端分离的 monorepo，通过 API-Gateway 作为对外的唯一入口网关，利用 DDD 对业务和技术需求做了拆分

```
IM/
├── api/                          # Proto 定义和生成代码
│   ├── proto/im/v1/              # *.proto 源文件
│   └── gen/im/v1/                # 生成的 Go 代码
│
├── services/                     # 微服务
│   ├── api_gateway/              # HTTP API 网关（端口 8080）
│   ├── identity_service/         # 身份认证（端口 9080）
│   ├── conversation_service/     # 会话管理（端口 9081）
│   ├── message_service/          # 消息服务（端口 9082）
│   ├── delivery_service/         # 消息投递（端口 8083）
│   ├── presence_service/         # 在线状态（端口 9084）
│   └── file_service/             # 文件服务（端口 9085）
│
├── pkg/                          # 共享库
│   ├── zlog/                     # 日志模块
│   ├── constants/                # 常量
│   └── enum/                     # 枚举
│
├── deploy/                       # 部署配置
│   ├── docker-compose.dev.yml    # 本地开发
│   ├── docker-compose.prod.yml   # 生产环境
│   └── sql/schema.sql            # 数据库脚本
│
└── go.work                       # Go workspace
```

### 服务端口分配

| 服务 | HTTP Port | gRPC Port | 说明 |
|------|-----------|-----------|------|
| API Gateway | 8080 | - | 统一入口网关 |
| Identity Service | 8081 | 9080 | 身份认证、用户管理 |
| Conversation Service | - | 9081 | 会话管理（仅 gRPC） |
| Message Service | 8083 | 9082 | 消息收发 |
| Delivery Service | 8084 | - | 消息投递、WebSocket |
| Presence Service | - | 9084 | 在线状态（仅 gRPC） |
| File Service | 8085 | 9085 | 文件上传 |

### 服务说明
#### api_gateway
HTTP API 统一入口，负责：
- 路由转发到各个微服务
- JWT Token 认证
- 请求限流与熔断

#### identity_service
身份认证服务，负责：
- 用户注册、登录
- JWT Token 签发与刷新
- 联系人管理（好友申请、好友列表）
- 用户资料管理

#### conversation_service
会话管理服务，负责：
- 创建单聊/群聊会话
- 会话成员管理
- 会话信息维护

#### message_service
消息服务，负责：
- 消息发送与存储
- 消息历史查询
- 已读状态管理
- 消息撤回
- 发布消息事件到 Kafka

#### delivery_service
消息投递服务，负责：
- 消费 Kafka 消息事件
- 通过 WebSocket 实时推送给在线用户
- 离线消息存储

#### presence_service
在线状态服务，负责：
- 用户上下线状态管理
- 批量查询在线状态

#### file_service
文件服务，负责：
- 生成 MinIO 预签名上传 URL
- 文件元数据管理


#### 待整理
##### 1. Redis Lua 脚本原子递增（消息序列号）
**文件位置**: `services/message_service/internal/adapters/out/redis/sequence_repo.go`

```go
// 核心实现：使用 Lua 脚本保证原子性
luaScript := `
local seq = redis.call('HINCRBY', KEYS[1], 'max_seq', 1)
if ARGV[1] ~= '' then
    redis.call('HSET', KEYS[1], 'msg_' .. ARGV[1], seq)
end
return seq
`
```

##### 2. Timeline 写扩散缓存
**文件位置**: `services/message_service/internal/adapters/out/redis/timeline_repo.go`

核心功能：
- ZSet 存储消息索引
- 支持分页获取
- 自动过期清理
- 批量添加消息

##### 3. 读扩散 Inbox 收件箱
**文件位置**: `services/message_service/internal/adapters/out/redis/inbox_repo.go`

核心功能：
- 用户收件箱管理
- Lua 脚本批量获取
- 会话未读计数
- 已读位置追踪

##### 4. Kafka 死信队列 & 可靠消费
**文件位置**: `services/delivery_service/internal/adapters/out/mq/reliable_consumer.go`

核心功能：
- 3 次重试机制
- 指数退避策略
- 自动转移死信队列
- 手动确认模式

```go
type ReliableConsumer struct {
    maxRetries    int           // 默认 3 次
    retryInterval time.Duration // 默认 1 秒
    dlqSuffix     string        // 死信队列后缀 "-dlq"
}
```

##### 5. ACK 机制（消息确认）
**文件位置**: 
- `services/delivery_service/internal/adapters/out/redis/pending_ack_repo.go`
- `services/delivery_service/internal/application/delivery.go`

核心功能：
- 待确认消息存储
- 超时重传机制（10秒）
- 批量 ACK 支持
- 已读状态同步

##### 6. Push-Pull 混合同步
**文件位置**: 
- `services/delivery_service/internal/adapters/out/redis/sync_state_repo.go`
- `services/delivery_service/internal/application/delivery.go`

核心功能：
- 实时 Push（在线用户）
- 离线 Pull（历史消息）
- 同步位置记录
- 增量拉取支持

##### 7. WebSocket 服务器
**文件位置**: `services/delivery_service/internal/adapters/in/ws/ws_server.go`

核心功能：
- JWT 认证
- 心跳检测（30秒）
- 连接管理
- 房间广播
- 消息分发

##### 8. 全局在线路由（多实例）
**文件位置**: `services/delivery_service/internal/adapters/out/redis/online_user_repo.go`

核心功能：
- Redis 分布式存储
- 支持多实例部署
- 用户所在实例查找
- 自动过期清理

##### 9. WebRTC 信令服务
**文件位置**: `services/delivery_service/internal/application/signaling.go`

核心功能：
- Offer/Answer 交换
- ICE Candidate 转发
- 通话状态机
- 超时处理（30秒）

支持的消息类型：
- `call_offer` - 发起呼叫
- `call_answer` - 接听呼叫
- `call_ice` - ICE Candidate
- `call_hangup` - 挂断

---

## 快速开始
### 本地开发
#### 克隆项目并安装依赖

```bash
# 克隆项目
git clone https://github.com/EthanQC/IM.git
cd IM

# 下载所有模块依赖
go work sync
go mod download
```

#### 拉取镜像并启动 Docker 容器

```bash
cd deploy
docker compose -f docker-compose.dev.yml up -d
```

等待所有容器启动（首次需要拉取镜像，可能需要几分钟）：

```bash
# 检查容器状态
docker ps

# 应该看到 5 个容器：im_mysql, im_redis, im_kafka, im_zookeeper, im_minio
```

**基础设施服务信息：**

| 服务 | 端口 | 访问地址 | 用户名 / 密码 |
|------|------|----------|---------------|
| MySQL | 3306 | localhost:3306 | root / imdev |
| Redis | 6379 | localhost:6379 | 无密码 |
| Kafka | 29092 | localhost:29092 | - |
| Zookeeper | 2181 | localhost:2181 | - |
| MinIO API | 9000 | localhost:9000 | admin / admin123 |
| MinIO 控制台 | 9001 | http://localhost:9001 | admin / admin123 |

#### 启动微服务

所有服务默认加载 `configs/config.dev.yaml` 配置文件，启动命令统一为：

```bash
# 启动各微服务（每个服务在独立终端窗口中运行）

# 1. Identity Service (身份认证)
cd services/identity_service && go run cmd/main.go

# 2. Conversation Service (会话管理)
cd services/conversation_service && go run cmd/main.go

# 3. Message Service (消息服务)
cd services/message_service && go run cmd/main.go

# 4. Delivery Service (消息投递/WebSocket)
cd services/delivery_service && go run cmd/main.go

# 5. Presence Service (在线状态)
cd services/presence_service && go run cmd/main.go

# 6. File Service (文件服务)
cd services/file_service && go run cmd/main.go

# 7. API Gateway (网关 - 最后启动)
cd services/api_gateway && go run cmd/main.go cmd/handlers.go
```

可以通过设置环境变量 `APP_ENV` 切换环境，如 `APP_ENV=prod` 使用生产配置

#### 访问 API 文档

打开浏览器访问：**http://localhost:8080/swagger**

在 Swagger UI 中可以：
- 查看所有 API 接口
- 点击 "Try it out" 直接测试
- 需要认证的接口，先登录获取 token，然后点击 "Authorize" 按钮输入

### 部署上云
#### 使用 Docker Compose

```bash
cd deploy

# 复制环境变量文件
cp .env.example .env

# 编辑 .env 文件，设置安全的密码
vim .env

# 启动所有服务
docker compose -f docker-compose.prod.yml up -d
```

#### 环境变量

| 变量 | 说明 |
|------|------|
| MYSQL_ROOT_PASSWORD | MySQL root 密码 |
| REDIS_PASSWORD | Redis 密码 |
| JWT_SECRET | JWT 签名密钥 |
| MINIO_ROOT_USER | MinIO 管理员用户名 |
| MINIO_ROOT_PASSWORD | MinIO 管理员密码 |

---

## API 接口
#### 认证（无需 Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/register | 用户注册 |
| POST | /api/auth/login | 用户登录 |
| POST | /api/auth/refresh | 刷新 Token |

#### 用户

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/users/me | 获取当前用户资料 |
| PUT | /api/users/me | 更新用户资料 |

#### 联系人

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/contacts | 获取联系人列表 |
| POST | /api/contacts/apply | 发送好友申请 |
| POST | /api/contacts/handle | 处理好友申请 |
| DELETE | /api/contacts/:id | 删除联系人 |

#### 会话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/conversations | 获取会话列表 |
| GET | /api/conversations/:id | 获取会话详情 |
| PUT | /api/conversations/:id | 更新会话 |
| POST | /api/conversations | 创建会话 |

#### 消息

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/messages | 发送消息 |
| GET | /api/messages/history | 获取历史消息 |
| POST | /api/messages/read | 标记已读 |
| POST | /api/messages/:id/revoke | 撤回消息 |

#### 在线状态

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/presence | 批量查询在线状态 |

#### 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/files/upload | 获取上传 URL |
| POST | /api/files/complete | 完成上传 |

#### WebSocket

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /ws | WebSocket 连接端点 |

---

## 常用命令
#### 进程管理

```bash
# 查看占用 8080 端口的进程
lsof -i :8080

# 查看所有 Go 进程
ps aux | grep "go run"

# 停止占用 8080 端口的进程
kill $(lsof -t -i :8080)

# 停止所有 Go 进程
pkill -f "go run"
```

#### Docker 容器管理

```bash
# 查看运行中的容器
docker ps

# 查看所有容器（包括已停止的）
docker ps -a

# 查看容器日志
docker logs im_mysql
docker logs im_kafka

# 停止所有容器
cd deploy && docker compose -f docker-compose.dev.yml down

# 停止并删除所有数据（重新开始）
cd deploy && docker compose -f docker-compose.dev.yml down -v

# 重新启动
cd deploy && docker compose -f docker-compose.dev.yml up -d
```

#### 依赖管理

```bash
# 同步 workspace
go work sync

# 下载依赖
go mod download

# 整理依赖
go mod tidy
```

#### 常见问题
##### Q: 端口被占用怎么办？

```bash
# 查看占用端口的进程
lsof -i :8080

# 杀死进程
kill -9 <PID>
```

##### Q: MySQL 连接失败？

等待 MySQL 完全启动（约 30 秒）：
```bash
docker logs im_mysql  # 查看日志
```

##### Q: Swagger 页面打不开？

确保是从 `services/api_gateway/cmd` 目录启动的：
```bash
cd services/api_gateway/cmd
go run main.go handlers.go -config ../configs/config.dev.yaml
```

##### Q: 如何完全重置环境？

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