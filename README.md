# 项目概述
## 简介
这是一个前后端分离的即时通讯项目，具备后台管理、单聊群聊、联系人管理、多种消息（文本 / 文件 / 视频）处理、离线消息处理以及音视频通话等功能，旨在打造类似微信的聊天体验

这是一个全面的即时通讯解决方案，支持：

* 一对一和群聊通信
* 联系人管理
* 消息历史记录和离线消息处理
* 文件共享
* 通过 WebRTC 进行音频/视频通话
* 用于用户和组管理的管理面板

技术选型方面待改造：
* 添加 sql 脚本自动迁移
* 搭本地开发、测试和生产这三个环境
* 日志模块继续用 zap 库，但要扩充一下现有的，并增加监控告警的配合
* http 响应格式改进，增加更多状态码或中间件
* http_server 目前是所有的路由注册入口，考虑更换整体架构（ddd好像还不错）
* 配置 ssl 来实现公网和跨域的 https 访问（证书配置相关）

业务开发方面待改造：
* service 层和 dao 层未解耦，后续直接考虑配合整体更换架构
* 登录方面手机验证码登录模块要考虑一下怎么大改，现在是用阿里云来实现的

项目目前模块架构：

* config 配置文件模块
  * 目前主要用 toml
  * 配合一个第三方库写了一套接口来实现的读取后端配置
* 基于 zap 库实现的日志模块
  * 支持日志切割和调用链信息
  * 以 json 格式输出
* redis 缓存模块
  * 主要用来缓存手机六位随机验证码和五种列表（联系人用户、我创建的群聊、我加入的群聊、用户会话、群聊会话）
  * 可以顺带着好好学一遍 redis 下来
* Kafka 消息队列模块
  * 主要是用来流量削峰的，在高流量时让服务器能顺利处理大量消息
  * 可以顺带着好好学一遍 Kafka 和消息队列下来（可以顺便再看看要不要换其他的消息队列工具）

重构后预期目录：

```
IM/                                # Monorepo 根目录
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
│   │   │   │   │   ├── phone.go          # 手机号值对象
│   │   │   │   │   └── password.go       # 密码值对象
│   │   │   │   └── service/              
│   │   │   │       └── auth_domain_service.go  # 领域服务
│   │   │   │
│   │   │   ├── application/               # 应用层 
│   │   │   │   ├── auth/
│   │   │   │   │   ├── generate_token.go     # Token生成用例
│   │   │   │   │   ├── verify_token.go       # Token校验用例
│   │   │   │   │   └── refresh_token.go      # Token刷新用例
│   │   │   │   └── sms/
│   │   │   │       ├── send_code.go          # 发送验证码用例
│   │   │   │       └── verify_code.go        # 验证码校验用例
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
│   │   ├── config/
│   │   │   └── config.go       # 日志配置结构
│   │   ├── core/
│   │   │   ├── level.go        # 日志级别定义
│   │   │   └── field.go        # 日志字段定义
│   │   ├── alert/
│   │   │   ├── alerter.go      # 告警接口定义
│   │   │   └── implementations/
│   │   │       ├── email.go    # 邮件告警实现
│   │   │       └── webhook.go  # Webhook告警实现
│   │   ├── metrics/
│   │   │   └── prometheus.go   # Prometheus指标收集
│   │   ├── formatter/
│   │   │   ├── formatter.go    # 格式化接口
│   │   │   ├── json.go        # JSON格式化
│   │   │   └── text.go        # 文本格式化
│   │   ├── output/
│   │   │   ├── writer.go      # 输出接口
│   │   │   ├── file.go        # 文件输出
│   │   │   └── console.go     # 控制台输出
│   │   ├── logger.go          # 主日志实现
│   │   └── options.go         # 日志选项配置
│   ├── errors/                       # 统一错误定义
│   ├── config/                       # 配置加载工具
│   ├── utils/
│   │   └── random/                   # 随机数工具
│   └── constants/                    # 常量定义
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

+ 前端：Vue3、Vue Router、Vuex、WebSocket、Element - UI 等
+ 后端：Go、Gin、GORM、GoRedis、WebSocket、Kafka、WebRTC、Zap 日志库等

# 项目结构

## 后端

```
IM/
├── api/
│   └── v1/
│       └── chatroom_controller.go
│       └── controller.go
│       └── group_info_controller.go
│       └── message_controller.go
│       └── session_controller.go
│       └── user_contact_controller.go
│       └── user_info_controller.go
│       └── ws_controller.go
├── cmd/
│   └── kama-chat-server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── dao/
│   │   └── gorm.go
│   ├── dto/
│   │   ├── request/
│   │   │   └── ......
│   │   └── respond/
│   │   │   └── ......
│   ├── https_server/
│   │   └── https_server.go
│   ├── model/
│   │   ├── contact_apply.go
│   │   ├── group_info.go
│   │   ├── message.go
│   │   ├── session.go
│   │   ├── user_contact.go
│   │   └── user_info.go
│   └── service/
│       ├── chat/
│       │   ├── client.go
│       │   ├── kafka_server.go
│       │   └── server.go
│       ├── gorm/
│       │   ├── chatroom_service.go
│       │   ├── group_info_service.go
│       │   ├── message_service.go
│       │   ├── session_service.go
│       │   ├── user_contact_service.go
│       │   └── user_info_service.go
│       ├── kafka/
│       │   └── kafka_service.go
│       ├── redis/
│       │   └── redis_service.go
│       └── sms/
│           ├── local/
│           │   └── user_info_service_local.go
│           └── auth_code_service.go
├── logs/
│   └── test.log
├── pkg/
│   ├── constants/
│   │   └── constants.go
│   ├── enum/
│   │   ├── contact/
│   │   ├── contact_apply/
│   │   ├── group_info/
│   │   ├── message/
│   │   ├── session/
│   │   └── user_info/
│   ├── ssl/
│   │   ├── xxx.pem
│   │   ├── xxx-key.pem
│   │   └── tls_handler.go
│   ├── util/
│   │   └── random/
│   │       └── random_int.go
│   └── zlog/
│       └── logger.go
├── configs/
│   ├── config.toml
│   └── config_local.toml
├── static/
│   ├── avatars/
│   │   └── ......
│   └── files/
│   │   └── ......
├── web/
│   └── (前端项目结构)
├── .gitignore
├── go.mod
├── go.sum
└── README.md

```

## 前端

```
web/chat-server/
├── src/
│   ├── assets/
│   │   ├── cert/
│   │   │   ├── xxx.pem
│   │   │   ├── xxx-key.pem
│   │   │   └── mkcert.exe
│   │   ├── css/
│   │   │   └── chat.css
│   │   ├── img/
│   │   │   └── chat_server_background.jpg
│   │   ├── js/
│   │   │   ├── random.js
│   │   │   └── valid.js
│   ├── components/
│   │   ├── ContactListModal.vue
│   │   ├── DeleteGroupModal.vue
│   │   ├── DeleteUserModal.vue
│   │   ├── DisableGroupModal.vue
│   │   ├── DisableUserModal.vue
│   │   ├── Modal.vue
│   │   ├── NavigationModal.vue
│   │   ├── SetAdminModal.vue
│   │   ├── SmallModal.vue
│   │   └── VideoModal.vue
│   ├── router/
│   │   └── index.js
│   ├── store/
│   │   └── index.js
│   ├── views/
│   │   ├── access/
│   │   │   ├── Login.vue
│   │   │   ├── Register.vue
│   │   │   └── SmsLogin.vue
│   │   ├── chat/
│   │   │   ├── contact/
│   │   │   │   ├── ContactChat.vue
│   │   │   │   └── ContactList.vue
│   │   │   ├── session/
│   │   │   │   └── SessionList.vue
│   │   │   └── user/
│   │   │       └── OwnInfo.vue
│   │   ├── manager/
│   │       └── Manager.vue
│   ├── App.vue
│   └── main.js
├── .gitignore
├── package.json
├── README.md
└── vue.config.js
```

# 功能特性
1. 即时通讯功能
   + 单聊与群聊：支持一对一私密聊天和群组聊天，消息实时推送。
   + 联系人管理：可添加、删除、拉黑联系人，处理好友申请等。
   + 消息类型：支持文本、文件、音视频等多种类型消息的发送与接收。
   + 离线消息处理：确保用户离线时消息不丢失，上线后可正常接收。
2. 音视频通话：基于 WebRTC 实现 1 对 1 音视频通话，包括发起、拒绝、接收、挂断通话等功能。
3. 后台管理：具备后台管理界面，靓号用户可进行人员管控等维护操作。
4. 安全与验证：登录注册采用 SMS 短信验证方式，并支持 SSL 加密，保障用户信息安全。
5. 后台mysql数据库：使用 GORM 进行数据库操作，确保数据持久化存储。
6. 日志记录：使用 Zap 日志库记录系统运行日志，便于问题排查与性能监控。
7. 消息队列：使用 Kafka 处理消息队列，确保消息的高效传输与处理。
8. redis缓存：使用 GoRedis 进行缓存操作，提高系统性能。
9. WebSocket：使用 WebSocket 实现实时消息推送，保证消息的实时性。

# 安装与运行
此次安装运行为一键部署，即可在Ubuntu22.04的云服务器上部署上线，公网都可以访问。
在执行脚本代码之前，需要做一些前置准备。
![](docs/image/3.png)
![](docs/image/4.png)
把端口3306（mysql）, 6379（redis）, 443（前端访问）, 80（云服务器http访问）, 22（ssh）, 3478（turn服务器，用于音视频公网转发）, 8000（后端访问）等端口开放。
![](docs/image/5.png)
打开前端src/views/chat/contact/ContactChat.vue，找到ICE_CFG配置，更新对应的turn服务器的相关配置。turn服务器就是你的云服务器。
如果需要本地通信的话，就需要把iceServers删掉，让ICE_CFG置空。
```toml
[mainConfig]
appName = "your app name"
host = "0.0.0.0"
port = 8000

[mysqlConfig]
host = "127.0.0.1"
port = 3306
user = "root"
password = "123456"
databaseName = "your database name"

[redisConfig]
host = "127.0.0.1"
port = 6379
password = ""
db = 0

[authCodeConfig]
accessKeyID = "your accessKeyID in alibaba cloud"
accessKeySecret = "your accessKeySecret in alibaba cloud"
signName = "阿里云短信测试"
templateCode = "SMS_154950909"

[logConfig]
logPath = "your log path"

[kafkaConfig]
messageMode = "channel"# 消息模式 channel or kafka
hostPort = "127.0.0.1:9092" # "127.0.0.1:9092,127.0.0.1:9093,127.0.0.1:9094" 多个kafka服务器
loginTopic = "login"
chatTopic = "chat_message"
logoutTopic = "logout"
partition = 0 # kafka partition
timeout = 1 # 单位秒

[staticSrcConfig]
staticAvatarPath = "./static/avatars"
staticFilePath = "./static/files"
```

你需要修改相应的后端配置文件中的内容。还需要先完成手机验证的功能，这篇需要看“后端开发”里的“手机验证”功能。

在这些都完成之后，就可以开始执行脚本代码了。

```bash
#!/bin/bash

# 更新系统软件包
sudo apt update && sudo apt upgrade -y

# 安装 MySQL
echo "Installing MySQL..."
sudo apt install mysql-server -y

# 配置 MySQL 安全
sudo mysql_secure_installation

# 启动并启用 MySQL 服务
sudo systemctl start mysql
sudo systemctl enable mysql

# 自动创建数据库
echo "Creating database 'kama_chat_server'..."
sudo mysql -u root -p <<EOF
CREATE DATABASE kama_chat_server;
EOF

# 安装 Redis
echo "Installing Redis..."
sudo apt install redis-server -y

# 配置 Redis
# sudo nano /etc/redis/redis.conf  # 修改 bind 127.0.0.1 改为 bind 0.0.0.0（如果需要外部访问）

# 启动并启用 Redis 服务
sudo systemctl restart redis
sudo systemctl enable redis

# 卸载旧版本 Node.js 和 npm，如果不是纯净版的Ubuntu的话
echo "Uninstalling previous versions of Node.js and npm..."
sudo apt remove --purge -y nodejs npm

# # 安装 Node.js 版本管理工具（nvm）
echo "Installing Node Version Manager (nvm)..."
rm -rf ~/.nvm
export NVM_NODE_MIRROR=https://npmmirror.com/mirrors/node/
export NVM_NPM_MIRROR=https://npmmirror.com/mirrors/npm/
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.5/install.sh | bash



# 设置 NVM_DIR 环境变量（避免重复写入 ~/.bashrc）
if ! grep -q "export NVM_DIR=~/.nvm" ~/.bashrc; then
    echo 'export NVM_DIR="$HOME/.nvm"' >> ~/.bashrc
    echo '[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # This loads nvm' >> ~/.bashrc
    echo '[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"  # This loads nvm bash_completion' >> ~/.bashrc
fi


# 手动设置 NVM_DIR 并加载 nvm
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # 加载 nvm
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"  # 加载 bash_completion

source ~/.bashrc

# 检查 nvm 是否存在
if ! command -v nvm &> /dev/null; then
    echo "nvm could not be found. Please ensure it is installed and added to your PATH."
    exit 1
fi

# 安装指定版本的 Node.js（例如 v16.x）
echo "Installing Node.js v16.x..."
nvm install 16
nvm use 16


# 加载环境变量
source ~/.bashrc

# 安装 Go
echo "Installing Go..."
wget https://mirrors.aliyun.com/golang/go1.20.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz

cp -r /usr/local/go/bin/* /usr/bin
# 设置 Go 环境变量
echo "Configuring Go environment..."

export PATH=$PATH:/usr/local/go/bin

 # 设置 Go 环境变量（避免重复写入 ~/.bashrc）
 if ! grep -q "export GOPATH=$HOME/go" ~/.bashrc; then
     echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
 fi
 source ~/.bashrc


# # 配置 Go 代理
echo "Configuring Go proxy..."
go env -w GOPROXY=https://goproxy.cn,direct

# 安装 Vue.js 开发环境
echo "Installing Vue.js development environment..."
sudo apt install npm -y



# 方案1：使用 npm 安装 Yarn
# sudo npm install -g yarn

# 方案2：使用cnpm 安装 Yarn
sudo npm install -g cnpm --registry=https://registry.npmjs.org
sudo cnpm install -g yarn

# 安装 Vue CLI
sudo cnpm install -g @vue/cli

# 重新安装项目依赖
cd ~/project/KamaChat/web/chat-server

yarn cache clean
rm -rf node_modules

yarn install # 会把package.json中所有依赖配置好的

#打包项目成dist，放到/var/www/html/，此时就可以通过云服务器的公网ip看到前端页面了
rm -rf /var/www/html/* 
rm -rf /root/project/KamaChat/web/chat-server/dist
yarn build
sudo cp -r /root/project/KamaChat/web/chat-server/dist/* /var/www/html # 改成自己的项目路径
sudo chmod -R 755 /var/www/html
sudo chown -R www-data:www-data /var/www/html

cd ~/project/KamaChat

# 安装 ssl 模块
echo "Installing ssl..."
sudo apt-get install openssl
sudo apt-get install libssl-dev

# # 创建根密钥，生成证书签名请求 (CSR)，创建根证书
openssl genrsa -out /etc/ssl/private/root.key 2048
openssl req -new -key /etc/ssl/private/root.key -out /etc/ssl/certs/root.csr
openssl x509 -req -in /etc/ssl/certs/root.csr -out /etc/ssl/certs/root.crt -signkey /etc/ssl/private/root.key -CAcreateserial -days 3650

# 生成服务器密钥，生成服务器证书签名请求 (CSR)，创建服务器证书扩展文件
openssl genrsa -out /etc/ssl/private/server.key 2048
openssl req -new -key /etc/ssl/private/server.key -out /etc/ssl/certs/server.csr
sudo nano v3.ext
# 内容如下
# authorityKeyIdentifier=keyid,issuer
# basicConstraints=CA:FALSE
# keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
# subjectAltName = @alt_names
# [alt_names]
# IP.1 = xxxxxxxxx # 你的云服务器地址

# # 使用根证书为服务器证书签名
openssl x509 -req -in /etc/ssl/certs/server.csr -CA /etc/ssl/certs/root.crt -CAkey /etc/ssl/private/root.key -CAcreateserial -out /etc/ssl/certs/server.crt -days 500 -sha256 -extfile v3.ext


# 打开Apache2配置文件
sudo nano /etc/apache2/sites-enabled/000-default.conf
# 添加如下内容
# <VirtualHost *:443>
#     ServerAdmin webmaster@localhost
#     DocumentRoot /var/www/html

#     SSLEngine on
#     SSLProxyEngine on

#     # 替换为您的自签名证书路径
#     SSLCertificateFile /etc/ssl/certs/server.crt
#     SSLCertificateKeyFile /etc/ssl/private/server.key

#     # 如果有中间证书，添加以下行
#     # SSLCertificateChainFile /path/to/your_intermediate.crt

#     ErrorLog ${APACHE_LOG_DIR}/error.log
#     CustomLog ${APACHE_LOG_DIR}/access.log combined

#     # 以下配置可选，用于启用 HTTP 到 HTTPS 重定向，也可以把这段添加到80端口那儿
#     <IfModule mod_rewrite.c>
#         RewriteEngine On
#         RewriteCond %{HTTPS} off
#         RewriteRule ^/?(.*) https://%{SERVER_NAME}%{REQUEST_URI} [R=301,L]
#     </IfModule>
# </VirtualHost>

# 启用ssl模块，启用ssl站点，重启服务
sudo a2enmod ssl
sudo a2ensite 000-default.conf
sudo systemctl restart apache2


# 配置turn服务器
echo "Installing coturn..."
sudo apt install coturn
sudo nano /etc/coturn/coturn.conf
# 配置以下参数
# listening-ip=0.0.0.0

# external-ip=xxxxx # 外部 IP 地址（替换为你的服务器公网 IP）

# listening-port=3478 # 监听端口

# user=username:password # 用户名和密码（替换为你的用户名和密码）

# tls-certificate=/etc/ssl/certs/server.crt # SSL 证书路径（如果需要加密通信）
# tls-private-key=/etc/ssl/private/server.key

sudo systemctl start coturn
sudo systemctl enable coturn


# 将后端打包部署
cd ~/project/KamaChat/cmd/kama_chat_server # 里面是main.go
go build -o kama_chat_backend main.go
sudo cp kama_chat_backend /usr/local/bin/

sudo nano /etc/systemd/system/kama_chat_backend.service
# 配置以下内容
# [Unit]
# Description=kama chat service
# After=network.target

# [Service]
# User=kama_chat  # 替换为你的用户名
# Group=kama_chat  # 替换为你的用户名
# WorkingDirectory=/root/project/KamaChat/cmd/kama_chat_server  # 替换为你的项目路径
# ExecStart=/usr/local/bin/kama_chat_backend  # 替换为你的可执行文件路径
# Restart=on-failure
# RestartSec=5

# [Install]
# WantedBy=multi-user.target

# 把后端服务起起来
sudo systemctl daemon-reload
sudo systemctl start kama_chat_backend
sudo systemctl enable kama_chat_backend

# 输出完成信息
echo "Deployment complete!"
```


在Ubuntu22.04云服务器上执行该脚本，它就会自动部署相关的依赖，并把go后端和vue前端部署到对应的位置，之后的访问可以通过https://xxxxx:443去访问。如果在前端访问后端的时候报错“NetWork error”时，可能后端还没部署好，可以重启一下。


# todoList

-  多对多群聊

-  nginx分布式部署

  
