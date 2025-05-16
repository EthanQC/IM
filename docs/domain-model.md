## 领域模型图
本图展示系统核心领域对象及其关系

#### 核心领域模型
```mermaid
classDiagram
    note for UserInfo "不变式:
        - username长度: 3-20
        - password长度: 6-20
        - phone必须符合手机号格式"
    
    note for GroupInfo "不变式:
        - name长度: 2-20
        - ownerId ∈ adminIds
        - adminIds ⊆ memberIds
        - memberIds.size ≥ 2"
    
    note for ChatRoom "不变式:
        - participants.size ≥ 2
        - lastMessageTime ≤ now()"

    %% 聚合根(使用粗边框)
    class UserInfo:::root {
      +String id
      +String username 
      +String password
      +String phone
      +String avatar
      +Boolean disable
      +DateTime createTime
      +DateTime updateTime
      +List<UserContact> contacts
      +List<Session> sessions
    }

    class GroupInfo:::root {
      +String id
      +String name
      +String avatar 
      +String description
      +Boolean disable
      +String ownerId
      +List<String> adminIds
      +List<String> memberIds
      +DateTime createTime
      +DateTime updateTime
    }

    class ChatRoom:::root {
      +String id
      +ChatType type
      +String lastMessageId
      +List<String> participants 
      +DateTime lastMessageTime
      +DateTime createTime
      +DateTime updateTime
    }

    %% 实体
    class Message:::entity {
      +String id
      +String senderId
      +String roomId
      +MessageContent content
      +Boolean isRead
      +DateTime sendTime
    }

    class UserContact:::entity {
      +String id
      +String userId
      +String contactId
      +String remark
      +DateTime createTime
    }

    class ContactApply:::entity {
      +String id
      +String userId
      +String targetId
      +String message
      +ApplyStatus status
      +DateTime createTime
    }

    %% 核心关系
    UserInfo "1" --o "0..*" UserContact : owns
    UserInfo "1" --> "0..*" ContactApply : processes
    UserInfo "1" --> "0..*" Message : sends
    GroupInfo "1" --> "0..*" UserInfo : has members
    ChatRoom "1" --* "0..*" Message : contains
    ChatRoom "1" --> "2..*" UserInfo : has participants

    %% 样式定义 - Material设计色板
    classDef root fill:#e3f2fd,stroke:#1976d2,stroke-width:4px
    classDef entity fill:#bbdefb,stroke:#1976d2,stroke-width:2px
```

#### 值对象模型
```mermaid
classDiagram
    note for MessageContent "不变式:
        - text.length ≤ 2000
        - type必须是合法的消息类型"
    
    note for FileAttachment "不变式:
        - size ≤ 50MB
        - type必须是支持的文件类型"
    
    note for AuthCode "不变式:
        - code长度 = 6
        - expireTime > now()
        - phone必须符合手机号格式"

    %% 值对象
    class MessageContent:::value {
      +MessageType type
      +String text
      +FileAttachment attachment
    }

    class FileAttachment:::value {
      +String url
      +String name
      +String type
      +Long size
    }

    class AuthCode:::value {
      +String phone
      +String code
      +DateTime expireTime
      +Boolean isUsed
    }

    %% 关系
    MessageContent "1" --o "0..1" FileAttachment : may have

    %% 样式定义
    classDef value fill:#e8f5e9,stroke:#43a047,stroke-dasharray: 5 5
```

#### 领域服务模型
```mermaid
classDiagram
    note for ChatRoomService "职责:
        - 管理实时消息广播
        - 维护在线用户状态
        - 处理群组消息转发"
    
    note for FileService "职责:
        - 文件上传大小限制: 50MB
        - 支持类型: 图片/文档/音视频
        - 存储路径规范化"

    %% 领域服务
    class ChatRoomService:::service {
      +broadcastMessage(message: Message)
      +getOnlineUsers(): List<UserInfo>
      +getUserRooms(userId: String): List<ChatRoom>
    }

    class AuthService:::service {
      +generateAuthCode(phone: String)
      +verifyAuthCode(phone: String, code: String)
      +createToken(user: UserInfo)
      +validateToken(token: String)
    }

    class FileService:::service {
      +uploadFile(file: File): String
      +downloadFile(fileId: String): File
      +deleteFile(fileId: String): Boolean
    }

    %% 服务依赖关系
    ChatRoomService --> AuthService : validates user
    ChatRoomService --> FileService : handles attachments

    %% 样式定义
    classDef service fill:#fff3e0,stroke:#f57c00,stroke-width:2px,stroke-dasharray: 10 5
```

#### 说明

1. 聚合根:
- UserInfo: 用户信息聚合根
- GroupInfo: 群组信息聚合根 
- Session: 会话聚合根
- ChatRoom: 管理消息和会话

1. 实体:
- Message: 消息实体
- UserContact: 联系人关系实体
- ContactApply: 好友申请实体
- Notification: 系统通知实体
- FileUpload: 文件上传记录实体

1. 值对象:
- MessageContent: 消息内容值对象
- FileAttachment: 文件附件值对象
- AuthCode: 手机验证码
- Email: 邮箱值对象

1. 领域服务:
- ChatRoomService: 负责消息广播和在线用户管理
- WebRTCService: 处理音视频通话
- AuthService: 处理登录认证相关
- FileService: 处理文件上传下载
- NotificationService: 处理系统通知

1. 关键业务规则:
- 用户可以发送多种类型消息(文本/文件/视频)
- 支持一对一聊天和群组聊天
- 联系人关系需要双方确认
- 群组有所有者和管理员角色
- 用户可以使用手机验证码或账号密码登录
- 支持文件的上传、下载和管理
- 系统消息通知(如好友申请、群组邀请等)
- 邮箱验证功能
- 消息发送需要经过权限验证
- 文件上传有大小和类型限制
- 群组必须至少有2个成员
- 验证码有效期和使用限制