# Instant Messaging
本项目是**基于微服务架构的即时通讯系统**，采用 **DDD（领域驱动设计）+ 六边形架构**，支持下列功能：

* 单聊/群聊
* 联系人管理
* 多种消息类型（文本/文件/视频）
* 离线消息处理
* 文件共享
* 音视频通话

## 技术栈

* 后端
  * 语言
    * Go 1.24.2
    * Web 框架：Gin
    * ORM：GORM
    * go-redis
  * 服务间通信
    * 同步：gRPC
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

## 仓库目录

```
IM/
├── api/                          # 统一的 Proto 定义和生成代码
│   ├── proto/im/v1/              # *.proto 源文件（唯一入口）
│   │   ├── identity.proto        # 身份服务接口
│   │   ├── conversation.proto    # 会话服务接口
│   │   ├── message.proto         # 消息服务接口
│   │   ├── presence.proto        # 在线状态接口
│   │   ├── file.proto            # 文件服务接口
│   │   └── common.proto          # 公共类型
│   └── gen/im/v1/                # buf generate 生成的 Go 代码
│
├── services/                     # 微服务目录
│   ├── api_gateway/              # HTTP/WS 网关
│   ├── identity_service/         # 身份认证服务
│   ├── conversation_service/     # 会话服务
│   ├── message_service/          # 消息服务
│   ├── delivery_service/         # 消息投递服务
│   ├── presence-service/         # 在线状态服务
│   ├── file_service/             # 文件服务
│   └── media-signal-service/     # 音视频信令服务
│
├── pkg/                          # 跨服务共享库
│   ├── zlog/                     # 基于 zap 的日志模块
│   ├── constants/                # 常量定义
│   ├── enum/                     # 枚举类型
│   ├── util/                     # 工具函数
│   └── ssl/                      # TLS 证书
│
├── deploy/                       # 部署配置
│   ├── docker-compose.dev.yml    # 本地开发依赖
│   └── sql/                      # 数据库初始化脚本
│
├── web/chat-server/              # 前端代码（Vue3 + Vite）
├── KamaChat/                     # 参考代码快照（旧版单体）
├── go.work                       # Go workspace 配置
├── buf.yaml                      # buf lint/generate 配置
└── README.md                  
```


## 架构总览

本项目采用前后端分离的 monorepo，通过 API-Gateway 作为对外的唯一入口网关，利用 DDD 对业务和技术需求做了拆分，已有身份、聊天、消息、投递、在线、音视频和文件共七个微服务





#### api_gateway

#### identity_service

#### conversation_service

#### message_service

#### delivery_service

#### presence_service

#### media_signal_service

#### file_service





## 环境管理
- 服务级配置：`services/<service>/configs/{env}/<service>.yaml`（示例：`identity_service/configs/dev/identity_service.yaml`）。包含端口、DB/Redis/Kafka/SMS/TTL 等。
- 日志配置：各服务 `configs/{env}/zlog.yaml`。
- 依赖编排：`deploy/docker-compose.dev.yml` 用于本地；生产建议按环境拆分 `deploy/k8s/{env}/`（预留）。
- 秘钥/证书：生产放入密钥管理（K8s Secret/云 KMS），本地用 `.env` 或配置文件占位，不进版本库。

## 快速开始（本地开发）
1) 依赖：Go 1.24.2、buf CLI、Docker & Docker Compose。  
2) 启动本地依赖：
   ```bash
   docker compose -f deploy/docker-compose.dev.yml up -d
   ```
   - MySQL root 密码：`imdev`，默认库 `identity_service`
   - Redis: `localhost:6379`
   - Kafka: `localhost:9092`（自动建 topic）
   - MinIO: `localhost:9000` / Console `9001`（admin/admin123）
3) 初始化数据库（identity_service 示例）：
   ```bash
   mysql -h 127.0.0.1 -u root -pimdev < services/identity_service/db/migrations/0001_init_tables.sql
   ```
4) 生成 proto（如网络可用）：`buf generate`。  
5) 运行 identity_service MVP：
   ```bash
   cd services/identity_service
   go run ./cmd/main.go -config ./configs/dev/identity_service.yaml
   ```
   - gRPC 默认 9090，HTTP 端口见配置。
   - Register/联系人接口当前返回 Unimplemented；Login/Refresh 可用。
6) 网关及其他服务：尚未落地，按「实现路线」逐步补齐（Conversation→Message→Delivery→Presence→File→Media-Signal）。






## 其他需求
二、技术需求

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