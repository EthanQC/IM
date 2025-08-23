# IM
## 简介
本项目**前后端分离**，通过用户、认证、聊天、群组、文件、后台共六个微服务部署在云端服务器，每个微服务均采用 **DDD 六边形架构**，具备**单聊群聊、联系人管理、多种消息处理（文本/文件/视频）、离线消息处理、文件共享、音视频通话和后台管理**等功能

立即体验：（待部署后补充域名）

## 文件目录

```
IM/                                   # Monorepo 根目录
├── services/                         # 所有微服务
│   ├── user-service/                 # 用户服务（边界上下文：User）
│   │   ├── cmd/
│   │   │   └── user-server/
│   │   │       └── main.go           # 启动入口
│   │   ├── internal/
│   │   │   ├── domain/               # 领域层
│   │   │   │   ├── entity/
│   │   │   │   │   └── user.go       # User 实体
│   │   │   │   ├── vo/
│   │   │   │   │   └── email.go      # Email 值对象
│   │   │   │   └── service/
│   │   │   │       └── user_domain_service.go  # 跨实体业务（如密码重置）
│   │   │   ├── application/          # 应用层（用例编排）
│   │   │   │   ├── register_user.go
│   │   │   │   └── login_user.go
│   │   │   ├── ports/                # 端口层（接口定义）
│   │   │   │   ├── in/
│   │   │   │   │   └── user_usecase.go         # RegisterUserUseCase, LoginUseCase
│   │   │   │   └── out/
│   │   │   │       ├── user_repository.go     # UserRepo 接口
│   │   │   │       └── auth_service.go        # TokenService 接口
│   │   │   └── adapters/             # 适配器层（具体实现）
│   │   │       ├── in/
│   │   │       │   ├── http/
│   │   │       │   │   └── user_controller.go
│   │   │       │   └── grpc/
│   │   │       │       └── user_grpc.go
│   │   │       └── out/
│   │   │           ├── db/
│   │   │           │   └── gorm_user_repo.go
│   │   │           └── auth/
│   │   │               └── satoken_adapter.go
│   │   ├── configs/
│   │   │   ├── config.dev.yaml
│   │   │   ├── config.test.yaml
│   │   │   └── config.prod.yaml
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── chat-service/                 # 聊天服务（边界上下文：Chat）
│   │   ├── cmd/chat-server/main.go
│   │   ├── internal/
│   │   │   ├── domain/
│   │   │   │   ├── entity/
│   │   │   │   │   └── message.go
│   │   │   │   ├── vo/
│   │   │   │   │   └── message_content.go
│   │   │   │   └── service/
│   │   │   │       └── chat_room_service.go    # 跨实体逻辑：群组广播
│   │   │   ├── application/
│   │   │   │   ├── send_message.go
│   │   │   │   └── get_history.go
│   │   │   ├── ports/
│   │   │   │   ├── in/
│   │   │   │   │   └── chat_usecase.go
│   │   │   │   └── out/
│   │   │   │       ├── message_repository.go
│   │   │   │       └── event_publisher.go
│   │   │   └── adapters/
│   │   │       ├── in/
│   │   │       │   ├── http/
│   │   │       │   │   └── chat_controller.go
│   │   │       │   └── ws/
│   │   │       │       └── ws_adapter.go
│   │   │       └── out/
│   │   │           ├── db/
│   │   │           │   └── gorm_message_repo.go
│   │   │           └── mq/
│   │   │               └── kafka_publisher.go
│   │   ├── configs/
│   │   │   ├── config.dev.yaml
│   │   │   ├── config.test.yaml
│   │   │   └── config.prod.yaml
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── group-service/               # 群组服务
│   │   └── …
│   │
│   ├── file-service/                # 文件服务
│   │
│   ├── auth-service/                # 认证服务
│   │   ├── cmd/
│   │   │   └── auth-server/
│   │   │       └── main.go                 # 服务启动入口
│   │   ├── internal/
│   │   │   ├── domain/                     # 领域层
│   │   │   │   ├── entity/
│   │   │   │   │   ├── auth_token.go      # Token实体
│   │   │   │   │   └── auth_code.go       # 验证码实体  
│   │   │   │   ├── vo/                    # 值对象
│   │   │   │   │   ├── phone.go           # 手机号值对象
│   │   │   │   │   └── password.go        # 密码值对象
│   │   │   │   └── service/              
│   │   │   │       └── auth_domain_service.go  # 领域服务
│   │   │   │
│   │   │   ├── application/               # 应用层 
│   │   │   │   ├── auth/
│   │   │   │   │   ├── generate_token.go  # Token生成用例
│   │   │   │   │   ├── verify_token.go    # Token校验用例
│   │   │   │   │   └── refresh_token.go   # Token刷新用例
│   │   │   │   └── sms/
│   │   │   │       ├── send_code.go       # 发送验证码用例
│   │   │   │       └── verify_code.go     # 验证码校验用例
│   │   │   │
│   │   │   ├── ports/                     # 端口层
│   │   │   │   ├── in/                    # 入站端口
│   │   │   │   │   ├── auth_api.go        # 认证相关接口
│   │   │   │   │   └── sms_api.go         # 短信相关接口
│   │   │   │   └── out/                   # 出站端口
│   │   │   │       ├── token_repo.go      # Token仓储接口
│   │   │   │       └── sms_service.go     # 短信服务接口
│   │   │   │
│   │   │   └── adapters/                  # 适配器层
│   │   │       ├── in/
│   │   │       │   ├── http/              # HTTP适配器
│   │   │       │   │   ├── auth_handler.go
│   │   │       │   │   └── sms_handler.go
│   │   │       │   └── grpc/              # gRPC适配器
│   │   │       │       └── auth_server.go  
│   │   │       └── out/
│   │   │           ├── redis/             # Redis适配器
│   │   │           │   └── token_repo_impl.go
│   │   │           └── aliyun/            # 阿里云短信适配器
│   │   │               └── sms_service_impl.go
│   │   │
│   │   ├── configs/                        # 配置文件
│   │   │   ├── config.dev.yaml
│   │   │   ├── config.test.yaml  
│   │   │   └── config.prod.yaml
│   │   │
│   │   ├── api/                           # API定义
│   │   │   └── proto/
│   │   │       └── auth.proto
│   │   │
│   │   ├── pkg/                           # 工具包
│   │   │   ├── jwt/
│   │   │   │   └── jwt.go
│   │   │   └── errors/
│   │   │       └── auth_errors.go
│   │   │
│   │   ├── Dockerfile                    
│   │   └── go.mod
│   │
│   └── notification-service/        # 通知服务
│
├── pkg/                              # 跨服务共享库
│   ├── logger/                       # 日志封装
│   │   ├── config.go       # 日志配置结构
│   │   ├── core.go         # 日志核心实现
│   │   ├── ctx.go          # 上下文相关
│   │   ├── global.go       # 全局实例相关
│   │   ├── level.go        # 日志级别相关
│   │   ├── metrics.go      # Prometheus 指标收集
│   │   ├── middleware.go   # 中间件相关
│   │   ├── readme.md       # 日志模块使用介绍
│   │   └── writer.go       # 日志输出管理
│
├── charts/                           # Helm Charts 或 Kustomize
│   ├── user-service/
│   ├── chat-service/
│   └── group-service/
│
├── scripts/
│   ├── docker-compose.yml            # 本地一键跑通所有依赖和服务
│   ├── dev-run.sh                    # Shell 脚本并行启动多个 main.go
│   └── k8s/
│       ├── dev/
│       ├── test/
│       └── prod/
│
├── docs/
│   ├── context-map.md                # 上下文图
│   ├── domain-model.png              # 领域模型图
│   ├── api-spec.md                   # 接口文档
│   └── architecture-overview.md      # 架构总览
│
├── config/
│   ├── global-config.yaml            # 注册中心、网关、监控等全局配置
│   └── registry-config.yaml
│
├── .gitignore
└── README.md
```

## 技术栈

* 架构
  * DDD 六边形
  * 微服务
* 前端
  * Vue 3 + Vite + TypeScript + Pinia
  * Vue Router + Element Plus + TailwindCSS
* 后端
  * Go
    * Gin
    * GORM
  * 数据库
    * MySQL
    * Redis
  * 日志
    * 基于 Zap 库封装的全局日志模块
  * 音视频
    * WebRTC
  * WebSocket
  * gRPC
  * protobuf
  * Kafka
* 工程化与运维
  * Docker
  * k8s（Kubernetes）
  * Github Actions
  * 监控告警
    * OpenTelemetry
    * Prometheus
    * Grafana
    * Alertmanager

## api-gateway
IM 对外唯一入口，REST + WebSocket

负责：
* TLS/SSL、JWT 校验、限流、CORS、统一错误与日志（Zap），承载 WebSocket 长连（握手/心跳/断线重连）
* 把外部请求转发到内部 gRPC 服务；把 delivery 的“推送”发到对应的 WS 连接

## identity-service
认证 + 用户 + 联系人

负责：
* 账号
  * 注册/登录/登出、签发/续期/校验 JWT；黑白名单（封禁用户、白名单路由）
* 查看/更新用户资料
* 联系人/好友
  * 申请、同意/拒绝、删除、拉黑；被拉黑不允许建会话/发送消息
* Casbin 管控接口权限（支持正则），策略变更实时生效

## conversation-service
会话/成员；单聊和群聊统一

负责：
* 创建/查询会话；成员增删；角色/禁言；会话属性（标题、头像、置顶）
* 校验“创建会话是否允许”（比如是否被拉黑）

## message-service
消息写入/历史/已读；生产 Kafka

负责：
* 发消息
  * 生成会话内 seq、按 clientMsgId 幂等、同事务写 messages + outbox，并写入 Kafka im.messages（key=conversation_id，保证会话内顺序）
* 历史
  * GET /messages?conv_id&since_seq&limit（按 (conv_id, seq) 翻页）
* 已读
  * 更新 inbox.last_read_seq
* 文件消息
  * 与 file-service 配合，消息体只存对象键/尺寸/MIME

## delivery-service
消息分发/离线/重试/死信；消费 Kafka

负责：
* 从 im.messages 消费消息；查询 presence 得到在线成员在哪个网关节点
* 在线：通过网关把消息推送到对应 WS
* 离线：更新 inbox.last_delivered_seq，用户上线后按 seq 回放
* 失败重试（指数退避）；超限进 DLQ；对“热会话”做限速，防止推送风暴

## presence-service
在线状态/路由，支撑推送

负责：
* 记录：用户在哪个网关节点、最后心跳时间；提供“是否在线/最后活跃时间”的查询
* 供 delivery 使用，找到要推送到哪个网关；将来可做“typing/在线列表”

## media-signal-service
音视频信令

负责：
* 1v1 通话的信令：发起/接受/拒绝/挂断；SDP/ICE 的转发；校验双方是否有共同会话（向 conversation 查询）
* 统计：发起成功率、ICE 失败率、TURN 占比

## file-service
文件/图片/语音视频切片上传

负责：
* 生成 S3/MinIO 预签名 URL（支持分片/断点续传/tus 可选），上传走对象存储，不压后台带宽
* 上传完成回调 message-service 写入一条“文件消息”（只存对象键与元数据）
* 基本图片缩略图（可选）、类型白名单、大小上限；（选配）ClamAV 简单扫描


1. 即时通讯功能
   + 联系人管理：可添加、删除、拉黑联系人，处理好友申请等。
   + 消息类型：支持文本、文件、音视频等多种类型消息的发送与接收。
   + 离线消息处理：确保用户离线时消息不丢失，上线后可正常接收。
2. 音视频通话：基于 WebRTC 实现 1 对 1 音视频通话，包括发起、拒绝、接收、挂断通话等功能。
3. 后台管理：具备后台管理界面，靓号用户可进行人员管控等维护操作。
4. 安全与验证：支持 SSL 加密，保障用户信息安全。
5. 后台mysql数据库：使用 GORM 进行数据库操作，确保数据持久化存储。
6. 日志记录：使用 Zap 日志库记录系统运行日志，便于问题排查与性能监控。
7. 消息队列：使用 Kafka 处理消息队列，确保消息的高效传输与处理。
8. redis缓存：使用 GoRedis 进行缓存操作，提高系统性能。
9. WebSocket：使用 WebSocket 实现实时消息推送，保证消息的实时性。



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