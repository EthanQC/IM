# `auth-service`
`Auth-Service` 提供统一的身份认证与权限管理能力，包含登录认证、令牌中心、RBAC 授权、短信验证码等模块，对外暴露 HTTP / gRPC 两套接口

## 业务流程
#### 注册
用户进来会先注册，可以使用手机号+密码或用户名+密码这两种方式注册，如果是前者会提示填写用户名，如果是后者会提示填写手机号

注册时会对手机号、用户名和密码规则分别进行校验

手机号是 11 位的大陆手机号码，用户名可以中文可以英文，但不能包含特殊字符，也不能重复，密码是 6-20 位，必须包含数字和字母

如果手机号、用户名和密码哪个不符合规则，会提示用户重新填写

注册成功会给反馈，只有注册成功后才能登录

#### 登录
登录时是进行身份验证，解决你是谁的问题，会看用户的角色是什么

用户可以使用手机号+密码/手机号+短信验证码/用户名+密码三种方式登录，登录时同样会对手机号、用户名、密码和短信验证码进行校验

如果校验通过，后端服务器会返回一个 JWT 令牌

#### 角色与权限


#### 令牌


#### 黑白名单


#### 主流程
注册——登录——请求接口

## 架构总览

```mermaid
flowchart TD
  %% 用户侧
  client[用户 / 前端]:::actor

  %% Auth-Service（对外接口）
  subgraph Auth-Service
    direction TB
    %% --- 公开 API ---
    register[POST /register]
    login_pwd[POST /login - 密码]
    send_code[POST /sms/send]
    login_sms[POST /login - 短信]
    refresh[POST /refresh]
    logout[POST /logout]
    introspect[POST /introspect]
  end

  %% ========= Auth-Service（Application + Domain） =========
  subgraph Application_Layer
    direction TB
    vo_check[Phone / Password VO 校验]
    code_lifecycle[AuthCode 生命周期]
    user_status[UserStatus 黑名单检查]
    token_issue[AuthToken 颁发 / 刷新 / 撤销]
  end

  %% ========= Ports / 适配器 =========
  subgraph Ports_Out
    direction TB
    user_repo[(UserRepo\nMySQL)]
    token_repo[(TokenRepo\nRedis)]
    code_repo[(AuthCodeRepo\nRedis)]
    sms_client[(SMSClient\nAliyun)]
    policy_repo[(PolicyRepo\nMySQL Roles/Perms)]
  end

  %% ========= 基础设施 =========
  redis_token[(Redis<br/>refresh_tokens / revoked_list)]
  redis_code[(Redis<br/>sms_code)]
  mysql_users[(MySQL<br/>users)]
  mysql_policy[(MySQL<br/>roles, permissions)]
  aliyun_sms[(Aliyun SMS)]

  %% ========= Gateway / API 中间件 =========
  subgraph Gateway_or_Middleware
    direction TB
    whitelist[接口白名单?]
    parse_jwt[解析 & 校验 JWT]
    blacklist[用户黑名单?]
    casbin[(Casbin enforce<br/>角色 / 权限)]
    handler[业务 Handler]
    deny401[[401 / 403]]
  end

  %% ========= 注册流程 =========
  client --> register --> vo_check --> user_repo --> mysql_users -->|成功| register_done[[201 Created]]
  register_done --> client

  %% ========= 登录 - 密码 =========
  client --> login_pwd
  login_pwd --> vo_check
  vo_check --> user_repo
  user_repo --> user_status
  user_status --未封禁--> token_issue
  token_issue --> token_repo --> redis_token
  token_issue --> login_ok[[200 JSON: Access + Refresh]]
  login_ok --> client

  user_status --封禁--> login_block[[403 Account Blocked]] --> client

  %% ========= 发送验证码 =========
  client --请求验证码--> send_code
  send_code --> code_lifecycle --> code_repo --> redis_code
  code_lifecycle --> sms_client --> aliyun_sms --> sms_sent[[200 SMS Sent]]
  sms_sent --> client

  %% ========= 登录 - 短信 =========
  client --> login_sms
  login_sms --> code_repo
  code_repo --> code_lifecycle
  code_lifecycle --校验成功--> user_repo
  user_repo --> user_status
  user_status --未封禁--> token_issue
  token_issue --> token_repo & login_ok_sms[[200 Tokens]]
  login_ok_sms --> client

  %% ========= Refresh =========
  client --> refresh --> token_repo
  token_repo --> redis_token --> token_issue
  token_issue --> redis_token & refresh_ok[[200 New Tokens]]
  refresh_ok --> client

  %% ========= Logout =========
  client --> logout --> token_repo --> redis_token --> logout_ok[[200 Revoked]]
  logout_ok --> client

  %% ========= 受保护接口请求流程 =========
  client -.带 Access Token .-> whitelist
  whitelist --Yes--> handler --> resp_ok[[200/xxx]]
  whitelist --No--> parse_jwt
  parse_jwt --> blacklist
  blacklist --Yes--> deny401
  blacklist --No--> casbin
  casbin --允许--> handler --> resp_ok
  casbin --禁止--> deny401
  deny401 --> client
  resp_ok --> client

  %% ========= 样式 =========
  classDef actor fill:#f9f,stroke:#333,stroke-width:1px;
  class client actor;
```
