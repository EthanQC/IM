# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# 复制多模块依赖
COPY api/ ./api/
COPY pkg/ ./pkg/
COPY services/delivery_service/ ./services/delivery_service/

WORKDIR /app/services/delivery_service

# 禁用 go.work 使用模块独立构建
ENV GOWORK=off

# 下载依赖
RUN go mod download && go mod tidy

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/services/delivery_service/main .
COPY --from=builder /app/services/delivery_service/configs ./configs

EXPOSE 8084

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8084/health || exit 1

CMD ["./main", "-config", "/app/configs/config.prod.yaml"]
