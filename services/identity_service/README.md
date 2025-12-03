# identity_service
身份与鉴权服务（MVP）：签发/刷新 Access & Refresh Token，短信验证码登录，基础 gRPC/HTTP 入口，对外实现 `api/proto/im/v1/identity.proto`（部分接口为占位）。

## 开发运行
```bash
cd services/identity_service
go run ./cmd/main.go -config ./configs/dev/identity_service.yaml
```
- 依赖：MySQL、Redis、Kafka（可用 `deploy/docker-compose.dev.yml` 启动）；短信可先用占位配置。
- 默认 gRPC 端口 9090，HTTP 端口见配置。

## 现状
- Login/Refresh 已接线到用例；Register/联系人相关接口当前返回 Unimplemented。
- 认证 token 由本服务签发；用户信息/联系人需要后续持久化实现后再接入。
