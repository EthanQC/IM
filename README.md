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
  * Vue3、Vue Router、Vuex、WebSocket、Element - UI 
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
* 部署
  * Docker
  * k8s（Kubernetes）
  * Github Actions
* 监控告警
  * OpenTelemetry
  * Prometheus
  * Grafana
  * Alertmanager

## 用户服务



## 认证服务



## 聊天服务
支持一对一私密聊天和群组聊天，消息实时推送

* 单聊
* 群聊

## 群组服务



## 文件服务



## 后台服务








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
商品服务
- 创建商品（可选）
- 修改商品信息（可选）
- 删除商品（可选）
- 查询商品信息（单个商品、批量商品）
购物车服务
- 创建购物车
- 清空购物车
- 获取购物车信息
订单服务
- 创建订单
- 修改订单信息（可选）
- 订单定时取消（高级）
结算
- 订单结算
支付
- 取消支付（高级）
- 定时取消支付（高级）
- 支付
AI大模型
- 订单查询
- 模拟自动下单