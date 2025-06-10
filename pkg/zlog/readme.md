# 日志模块说明

本日志模块基于 Uber 开源的 Zap 日志库二次自定义封装而成，提供了**多目标输出**、**动态日志级别**、**Context 传递**、**指标埋点**等功能

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

## 快速上手
#### go.work 管理
假设你的项目结构如下：

    IM/
    ├── go.work
    ├── pkg/
    │   └── zlog/              ← 本模块
    └── services/
        └── myservice/         ← 你的微服务

在 `IM/` 根目录运行 `go work init ./pkg/zlog ./services/myservice`，此时在 `myservice` 下直接 `import "github.com/EthanQC/IM/pkg/zlog"` 即可引用本地日志模块

#### 使用示例
先在各个微服务的 `main` 文件中初始化：
```go
// 加载日志配置
cfgPath := "config/zlog.yaml"
cfg, err := zlog.LoadConfig(cfgPath)

if err != nil {
    panic("日志配置加载失败：" + err.Error())
}

// 初始化全局 logger
zlog.MustInitGlobal(*cfg)
defer zlog.Sync()

// 注册 Prometheus 指标（待具体实现）
// zlog.RegisterMetrics(prometheus.DefaultRegisterer)

// 加载 Gin 中间件
r := gin.New()
r.use(
    zlog.GinLogger(),
    gin.Recovery(),
)

// 动态调整日志级别
r.PUT("/log/level", zlog.LevelHTTPHandler())

// 暴露 Prometheus /metrics
// r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

基础使用:
```go
// 记录日志
zlog.Info("user login", 
    zlog.String("user_id", "123"),
    zlog.Int("login_count", 5),
)

zlog.Debug("开始处理任务", zlog.String("task_id", id))
```

带Context使用:
```go
logger := zlog.FromContext(ctx)
logger.Info("process message",
    zlog.String("msg_id", msgID),
    zlog.Any("payload", payload),
)
```

## 技术选型相关
#### 为什么不选择热加载和 Kafka

* 目前项目规模较小
* 热加载主要用途是无需重启服务，能动态调整配置
    * 但目前重启服务来加载新配置成本很低
    * 引入热加载会额外依赖文件监听和并发锁，增加复杂度
* Kafka 更适合大规模分布式日志收集
    * 不打算接 ElasticSearch

后续搭建告警和监控的时候会重新评估考虑