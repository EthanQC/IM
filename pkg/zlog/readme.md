# 日志模块说明

本日志模块基于 Uber 开源的 Zap 日志库二次自定义封装而成，提供了**多目标输出**、**动态日志级别**、**Context 传递**、**可选指标埋点**等功能

## 模块架构图

```mermaid
flowchart TB
    %% 定义子图样式
    classDef subgraphStyle fill:#f5f5f5,stroke:#666,stroke-width:2px
    classDef mainNodeStyle fill:#e1f5fe,stroke:#0288d1,stroke-width:2px 
    classDef configNodeStyle fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef outputNodeStyle fill:#e8f5e9,stroke:#388e3c,stroke-width:2px,stroke-dasharray: 5 5
    classDef globalNodeStyle fill:#ffebee,stroke:#c62828,stroke-width:2px

    %% 全局实例管理
    subgraph Global["全局实例"]
        direction TB
        Default["默认 Logger"] 
        Replace["替换全局 logger<br/>global.go"]
        Signal["信号处理<br/>优雅关闭"]
        Default --> Replace
        Replace --> Signal
        class Default,Replace,Signal globalNodeStyle
    end

    %% 服务启动流程
    subgraph Init["服务初始化"]
        direction TB
        Load("加载配置") --> Config["config.go"] 
        Config --> New("创建 Logger 实例")
        class Load,Config,New mainNodeStyle
    end

    %% 核心组件
    subgraph Core["核心组件"]
        direction TB
        Level("日志级别管理<br/>level.go") 
        Writer["输出管理<br/>writer.go"]
        Metric["指标收集<br/>metrics.go"]
        Main["核心逻辑<br/>core.go"]
        
        Level --> Main
        Main --> Writer
        Main --> Metric
        class Level,Writer,Metric,Main mainNodeStyle
    end

    %% 输出目标
    subgraph Output["输出目标"]
        Console[("控制台输出")]
        File[("文件输出<br/>lumberjack")]
        class Console,File outputNodeStyle
    end

    %% 中间件集成
    subgraph Middleware["中间件与上下文"]
        direction LR
        Mid["HTTP中间件<br/>middleware.go"] --> Ctx["Context传递<br/>ctx.go"]
        class Mid,Ctx mainNodeStyle
    end

    %% 动态控制
    subgraph Control["运行时控制"]
        direction TB
        HTTP["HTTP 接口<br/>/log/level"] --> LevelCtl["动态调整<br/>level.go"]
        Signal2["信号处理<br/>kill -HUP"] --> LevelCtl
        class HTTP,Signal2,LevelCtl configNodeStyle
    end

    %% 连接关系
    Init --> Core
    Core --> Output
    Middleware --> Core
    Control --> Core
    Global --> Core

    %% 应用整体样式
    class Init,Core,Output,Middleware,Control,Global subgraphStyle
```

## 核心功能
### 1. 全局实例管理 (global.go)
- 提供全局默认 logger
- 支持安全地替换全局实例
- 处理服务优雅关闭

### 2. 配置管理 (config.go) 
- 支持多环境配置
- 日志输出路径配置
- 日志级别配置
- 编码格式配置(JSON/Console)
- 文件切割配置

### 3. 日志级别管理 (level.go)
- 支持动态调整日志级别
- 提供HTTP接口动态修改
- 支持信号触发级别调整

### 4. 输出管理 (writer.go)
- 支持同时输出到多个目标
- 文件自动切割归档
- 并发安全的写入

### 5. 上下文集成 (ctx.go)
- 支持链路追踪
- Context传递关键信息
- 支持Fields扩展

### 6. 中间件支持 (middleware.go)
- 集成HTTP请求日志
- 记录请求耗时
- 支持自定义字段

### 7. 监控指标 (metrics.go)
- 统计日志数量
- 记录日志级别分布
- 监控写入延迟

## 使用示例

1. 基础使用:
```go
// 初始化全局 logger
zlog.MustInitGlobal(zlog.Config{
    Level:  "info",
    File: zlog.FileConfig{
        Path:   "logs/app.log",
        MaxSize: 100,    // MB
    },
})

// 记录日志
zlog.Info("user login", 
    zlog.String("user_id", "123"),
    zlog.Int("login_count", 5),
)
```

2. 带Context使用:
```go
logger := zlog.FromContext(ctx)
logger.Info("process message",
    zlog.String("msg_id", msgID),
    zlog.Any("payload", payload),
)
```

3. HTTP中间件:
```go
r := gin.New()
r.Use(zlog.GinLogger()) 
```

## 性能考虑
1. 使用sync.Pool复用对象
2. 支持采样记录
3. 异步写入选项
4. 批量写入缓冲

## 后续规划
1. 集成ELK支持
2. 完善告警机制
3. 增加更多监控指标
4. 支持日志采样
5. 增强链路追踪

## 注意事项
1. 正确设置日志级别避免性能问题
2. 定期清理归档日志
3. 合理配置文件切割大小
4. 避免频繁替换全局 logger
5. panic 前确保日志刷盘

## 技术选型相关

#### 为什么不选择热加载和 Kafka

* 目前项目规模较小
* 热加载主要用途是无需重启服务，能动态调整配置
    * 但目前重启服务来加载新配置成本很低
    * 引入热加载会额外依赖文件监听和并发锁，增加复杂度
* Kafka 更适合大规模分布式日志收集
    * 不打算接 ElasticSearch

后续搭建告警和监控的时候会重新评估考虑