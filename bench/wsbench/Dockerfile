# wsbench 压测工具镜像
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o wsbench .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/wsbench .

# 创建结果目录
RUN mkdir -p /results

ENTRYPOINT ["/app/wsbench"]
